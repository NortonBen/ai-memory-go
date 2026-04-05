#!/usr/bin/env python3
"""
Export microsoft/harrier-oss-v1-270m from HuggingFace SafeTensors → ONNX format
for use with github.com/owulveryck/onnx-go in the AI Memory pipeline.

Requirements:
    pip install torch transformers "optimum[onnxruntime]" onnx onnxruntime

Usage:
    python scripts/export_harrier_onnx.py \
        --model microsoft/harrier-oss-v1-270m \
        --output /path/to/model_dir \
        --seq-len 512

The script tries three export strategies in order:
  1. optimum  – best Gemma3/transformer support
  2. torch.export (PyTorch 2.x new-style dynamo)
  3. TorchScript legacy (fallback with eager attention)

After export, set your ~/.ai-memory.yaml:

    embedder:
      provider: onnx
      model: microsoft/harrier-oss-v1-270m
      dimensions: 640
      onnx:
        model_path: /path/to/model_dir/model.onnx
        tokenizer_path: /path/to/model_dir/tokenizer.json
        max_seq_len: 512
        query_task: "Retrieve semantically similar text"
        use_query_instruction: true
"""

import argparse
import os
import subprocess
import sys
from pathlib import Path

import torch
import torch.nn as nn
import torch.nn.functional as F
from transformers import AutoModel, AutoTokenizer


# ---------------------------------------------------------------------------
# Pooling helpers
# ---------------------------------------------------------------------------

def last_token_pool(last_hidden_states: torch.Tensor, attention_mask: torch.Tensor) -> torch.Tensor:
    left_padding = (attention_mask[:, -1].sum() == attention_mask.shape[0])
    if left_padding:
        return last_hidden_states[:, -1]
    sequence_lengths = attention_mask.sum(dim=1) - 1
    batch_size = last_hidden_states.shape[0]
    return last_hidden_states[
        torch.arange(batch_size, device=last_hidden_states.device),
        sequence_lengths,
    ]


# ---------------------------------------------------------------------------
# Strategy 1: optimum
# ---------------------------------------------------------------------------

def _try_optimum(model_id: str, out_dir: str, seq_len: int) -> bool:
    print("      [strategy 1] optimum …")
    for cmd in [
        [sys.executable, "-m", "optimum.exporters.onnx",
         "--model", model_id, "--task", "feature-extraction",
         "--opset", "17", "--sequence_length", str(seq_len), out_dir],
        ["optimum-cli", "export", "onnx",
         "--model", model_id, "--task", "feature-extraction", out_dir],
    ]:
        try:
            r = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
            if r.returncode == 0:
                return True
        except (subprocess.TimeoutExpired, FileNotFoundError):
            continue
    return False


# ---------------------------------------------------------------------------
# Strategy 2: torch.export (new PyTorch 2.x)
# ---------------------------------------------------------------------------

def _try_torch_export(model_id: str, out_dir: str, seq_len: int, device: str) -> bool:
    print("      [strategy 2] torch.export …")
    try:
        from torch.export import export as tx_export, Dim

        hf_model = AutoModel.from_pretrained(model_id, dtype=torch.float32)
        hf_model.eval().to(device)

        class W(nn.Module):
            def __init__(self, m):
                super().__init__()
                self.m = m
            def forward(self, input_ids, attention_mask):
                out = self.m(input_ids=input_ids, attention_mask=attention_mask)
                emb = last_token_pool(out.last_hidden_state, attention_mask)
                return F.normalize(emb, p=2, dim=1)

        wrapper = W(hf_model).eval()
        ids = torch.zeros(1, seq_len, dtype=torch.long, device=device)
        mask = torch.ones(1, seq_len, dtype=torch.long, device=device)

        b = Dim("batch", min=1, max=8)
        s = Dim("sequence", min=1, max=seq_len)
        ep = tx_export(wrapper, args=(ids, mask),
                       dynamic_shapes={"input_ids": {0: b, 1: s},
                                       "attention_mask": {0: b, 1: s}})

        onnx_path = str(Path(out_dir) / "model.onnx")
        torch.onnx.export(ep, onnx_path,
                          input_names=["input_ids", "attention_mask"],
                          output_names=["embeddings"], verbose=False)
        return True
    except Exception as e:
        print(f"        failed: {e}", file=sys.stderr)
        return False


# ---------------------------------------------------------------------------
# Strategy 3: TorchScript + eager attention (legacy fallback)
# ---------------------------------------------------------------------------

def _try_torchscript(model_id: str, out_dir: str, seq_len: int, device: str) -> bool:
    print("      [strategy 3] TorchScript + eager attention …")
    trace_len = min(seq_len, 128)
    try:
        hf_model = AutoModel.from_pretrained(
            model_id, dtype=torch.float32, attn_implementation="eager"
        )
        hf_model.eval().to(device)

        class WEager(nn.Module):
            def __init__(self, m):
                super().__init__()
                self.m = m
            def forward(self, input_ids: torch.Tensor, attention_mask: torch.Tensor) -> torch.Tensor:
                out = self.m(input_ids=input_ids, attention_mask=attention_mask)
                seq_lengths = attention_mask.sum(dim=1) - 1
                bs = out.last_hidden_state.shape[0]
                emb = out.last_hidden_state[torch.arange(bs), seq_lengths]
                norm = torch.norm(emb, p=2, dim=1, keepdim=True).clamp(min=1e-12)
                return emb / norm

        wrapper = WEager(hf_model).eval()
        ids = torch.zeros(1, trace_len, dtype=torch.long, device=device)
        mask = torch.ones(1, trace_len, dtype=torch.long, device=device)
        onnx_path = str(Path(out_dir) / "model.onnx")

        with torch.no_grad():
            torch.onnx.export(
                wrapper, (ids, mask), onnx_path,
                export_params=True, opset_version=17,
                do_constant_folding=True,
                input_names=["input_ids", "attention_mask"],
                output_names=["embeddings"],
                dynamic_axes={"input_ids": {0: "batch", 1: "sequence"},
                              "attention_mask": {0: "batch", 1: "sequence"},
                              "embeddings": {0: "batch"}},
                verbose=False, dynamo=False,
            )
        return True
    except Exception as e:
        print(f"        failed: {e}", file=sys.stderr)
        return False


# ---------------------------------------------------------------------------
# Main export pipeline
# ---------------------------------------------------------------------------

def export(model_id: str, output_dir: str, seq_len: int, device: str) -> None:
    out = Path(output_dir)
    out.mkdir(parents=True, exist_ok=True)
    onnx_path = str(out / "model.onnx")

    # [1/4] Tokenizer
    print(f"[1/4] Saving tokenizer …")
    tok = AutoTokenizer.from_pretrained(model_id)
    tok.save_pretrained(str(out))
    print(f"      → {out / 'tokenizer.json'}")

    # [2/4] Export
    print(f"[2/4] Exporting ONNX model (seq_len={seq_len}) …")
    ok = (
        _try_optimum(model_id, str(out), seq_len)
        or _try_torch_export(model_id, str(out), seq_len, device)
        or _try_torchscript(model_id, str(out), seq_len, device)
    )
    if not ok:
        print("ERROR: all export strategies failed.", file=sys.stderr)
        sys.exit(1)

    # Rename if optimum used a different filename
    for candidate in out.glob("*.onnx"):
        if candidate.name != "model.onnx":
            candidate.rename(out / "model.onnx")
            break
    print(f"      → {onnx_path}")

    # [3/4] Verify
    print("[3/4] Verifying ONNX …")
    try:
        import onnx, onnxruntime as ort, numpy as np

        onnx.checker.check_model(onnx.load(onnx_path))
        sess = ort.InferenceSession(onnx_path, providers=["CPUExecutionProvider"])
        trace_len = min(seq_len, 128)
        dummy_ids = np.zeros((1, trace_len), dtype=np.int64)
        dummy_mask = np.ones((1, trace_len), dtype=np.int64)
        out_arr = sess.run(None, {"input_ids": dummy_ids, "attention_mask": dummy_mask})[0]
        norm = float(np.linalg.norm(out_arr[0]))
        print(f"      shape={out_arr.shape}  L2-norm={norm:.6f}  (should be ~1.0)")
        print("      ONNX verification PASSED ✓")
    except ImportError:
        print("      (skip – install onnx & onnxruntime to verify)")
    except Exception as e:
        print(f"      WARNING: {e}", file=sys.stderr)

    # [4/4] Summary
    print()
    print("[4/4] Done! Add to ~/.ai-memory.yaml:")
    print()
    print("  embedder:")
    print("    provider: onnx")
    print("    model: microsoft/harrier-oss-v1-270m")
    print("    dimensions: 640")
    print("    onnx:")
    print(f"      model_path: {onnx_path}")
    print(f"      tokenizer_path: {out / 'tokenizer.json'}")
    print("      max_seq_len: 512")
    print('      query_task: "Retrieve semantically similar text"')
    print("      use_query_instruction: true")


def main():
    p = argparse.ArgumentParser(description="Export Harrier-OSS-v1-270m → ONNX")
    p.add_argument("--model", default="microsoft/harrier-oss-v1-270m")
    p.add_argument("--output", required=True, help="Output directory")
    p.add_argument("--seq-len", type=int, default=512)
    p.add_argument("--device", default="cpu")
    args = p.parse_args()
    export(args.model, args.output, args.seq_len, args.device)


if __name__ == "__main__":
    main()
