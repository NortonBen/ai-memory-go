#!/usr/bin/env python3
"""
Export a DeBERTa-v3 NER model to ONNX + tuỳ chọn quantize INT8.

RAM footprint so sánh (trên disk / ONNX Runtime RSS khi chạy):
  DeBERTa-v3-large   FP32  ~1.6 GB / ~2.5 GB   (mặc định cũ, chính xác nhất)
  DeBERTa-v3-base    FP32  ~0.4 GB / ~0.7 GB   ← khuyến nghị (đủ tốt cho CoNLL)
  DeBERTa-v3-large   INT8  ~0.4 GB / ~0.6 GB   (quantize, nhanh hơn nhưng kém hơn một chút)
  DeBERTa-v3-base    INT8  ~0.1 GB / ~0.2 GB   ← nhỏ nhất, tốt cho môi trường RAM giới hạn

Dùng với Harrier-OSS-v1-270m (1.0 GB) → tổng RSS:
  base FP32 + Harrier CPU : ~1.7 GB   ← mặc định khuyến nghị
  base INT8 + Harrier CPU : ~1.2 GB
  large FP32 + Harrier CPU: ~3.5 GB

Usage:
    # Khuyến nghị: DeBERTa-v3-base (nhỏ, nhanh, CoNLL-2003)
    python scripts/export_deberta_onnx.py \\
        --size base \\
        --output data/deberta-ner

    # Chính xác nhất: DeBERTa-v3-large
    python scripts/export_deberta_onnx.py \\
        --size large \\
        --output data/deberta-ner-large

    # Nhỏ nhất: base + INT8 quantize
    python scripts/export_deberta_onnx.py \\
        --size base --quantize \\
        --output data/deberta-ner-q

    # Dùng model tuỳ chỉnh
    python scripts/export_deberta_onnx.py \\
        --model Gladiator/microsoft-deberta-v3-large_ner_conll2003 \\
        --output data/deberta-ner-large

Output files:
    data/deberta-ner/
      model.onnx        — ONNX NER model (FP32 hoặc INT8)
      tokenizer.json    — SentencePiece Unigram tokenizer (for Go)
      labels.json       — {"0":"O","1":"B-PER","2":"I-PER",...}
"""
import argparse
import glob
import json
import os
import sys

# Map alias → HuggingFace model ID
_SIZE_MAP = {
    "base":  "Gladiator/microsoft-deberta-v3-base_ner_conll2003",
    "large": "Gladiator/microsoft-deberta-v3-large_ner_conll2003",
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--model",
        default="",
        help="HuggingFace model name or local path (takes precedence over --size)",
    )
    parser.add_argument(
        "--size",
        choices=["base", "large"],
        default="base",
        help="Model size alias: 'base' (~400 MB, khuyến nghị) hoặc 'large' (~1.6 GB). "
             "Bị ghi đè bởi --model nếu có.",
    )
    parser.add_argument("--output", default="data/deberta-ner")
    parser.add_argument("--seq-len", type=int, default=256,
                        help="Max sequence length (256 đủ cho NER, tiết kiệm RAM hơn 512)")
    parser.add_argument(
        "--quantize",
        action="store_true",
        help="Quantize INT8 sau khi export (giảm ~4x kích thước, tăng tốc CPU)",
    )
    args = parser.parse_args()

    # Resolve model name
    if not args.model:
        args.model = _SIZE_MAP[args.size]

    os.makedirs(args.output, exist_ok=True)

    # Redirect HF cache into workspace (sandbox-safe)
    os.environ["HF_HOME"] = os.path.join(os.getcwd(), "data", "hf_cache")
    os.environ["TRANSFORMERS_CACHE"] = os.path.join(os.getcwd(), "data", "hf_cache")

    print(f"[1/4] Loading model {args.model!r} …")
    from transformers import AutoTokenizer, AutoModelForTokenClassification
    import torch

    tokenizer = AutoTokenizer.from_pretrained(args.model)
    model = AutoModelForTokenClassification.from_pretrained(args.model)
    model.eval()

    # ── Label map ────────────────────────────────────────────────────────────
    id2label = getattr(model.config, "id2label", {})
    labels = {str(k): v for k, v in id2label.items()}
    if not labels:
        # Fall back to CoNLL-2003 defaults
        labels = {
            "0": "O",
            "1": "B-PER", "2": "I-PER",
            "3": "B-ORG", "4": "I-ORG",
            "5": "B-LOC", "6": "I-LOC",
            "7": "B-MISC", "8": "I-MISC",
        }
        print(f"    (no id2label in config — using CoNLL-2003 defaults)")

    labels_path = os.path.join(args.output, "labels.json")
    with open(labels_path, "w", encoding="utf-8") as f:
        json.dump(labels, f, ensure_ascii=False, indent=2)
    print(f"    Labels : {list(labels.values())}")

    # ── Tokenizer ─────────────────────────────────────────────────────────────
    print(f"\n[2/4] Saving tokenizer …")
    tokenizer.save_pretrained(args.output)
    print(f"    → {args.output}/tokenizer.json")

    # ── ONNX export ───────────────────────────────────────────────────────────
    print(f"\n[3/4] Exporting ONNX model …")
    target_path = os.path.join(args.output, "model.onnx")

    exported = False

    # Strategy 1: optimum (best for DeBERTa)
    try:
        from optimum.exporters.onnx import main_export

        print(f"    [strategy 1] optimum …")
        main_export(
            model_name_or_path=args.model,
            output=args.output,
            task="token-classification",
            opset=14,
        )
        # optimum may save the file with a different name
        candidates = glob.glob(os.path.join(args.output, "**", "*.onnx"), recursive=True)
        for c in candidates:
            if os.path.basename(c) != "model.onnx":
                import shutil
                shutil.move(c, target_path)
            break
        if os.path.exists(target_path):
            print(f"    → {target_path}")
            exported = True
        else:
            print("    optimum did not produce model.onnx — falling back")
    except Exception as e:
        print(f"    optimum failed: {e}")

    # Strategy 2: torch.onnx.export (TorchScript trace)
    if not exported:
        print(f"    [strategy 2] torch.onnx.export …")
        try:
            dummy = tokenizer(
                "Hello world this is a test sentence for NER export.",
                return_tensors="pt",
                max_length=args.seq_len,
                padding="max_length",
                truncation=True,
            )
            input_names = list(dummy.keys())
            dynamic_axes = {k: {0: "batch", 1: "seq"} for k in input_names}
            dynamic_axes["logits"] = {0: "batch", 1: "seq"}

            with torch.no_grad():
                torch.onnx.export(
                    model,
                    tuple(dummy[k] for k in input_names),
                    target_path,
                    input_names=input_names,
                    output_names=["logits"],
                    dynamic_axes=dynamic_axes,
                    opset_version=14,
                    dynamo=False,
                )
            print(f"    → {target_path}")
            exported = True
        except Exception as e:
            print(f"    torch.onnx.export failed: {e}")

    if not exported:
        print("FATAL: all export strategies failed.", file=sys.stderr)
        sys.exit(1)

    # ── INT8 Quantization (optional) ──────────────────────────────────────────
    if args.quantize:
        print(f"\n[3b] INT8 Dynamic Quantization …")
        try:
            from onnxruntime.quantization import quantize_dynamic, QuantType
            q_path = target_path.replace(".onnx", "_int8.onnx")
            quantize_dynamic(
                target_path,
                q_path,
                weight_type=QuantType.QInt8,
            )
            orig_mb = os.path.getsize(target_path) / 1e6
            q_mb = os.path.getsize(q_path) / 1e6
            print(f"    FP32 : {orig_mb:.1f} MB → INT8 : {q_mb:.1f} MB  "
                  f"(giảm {orig_mb/q_mb:.1f}x)")
            # Replace FP32 with INT8
            import shutil
            shutil.move(q_path, target_path)
            print(f"    → {target_path}  (INT8)")
        except Exception as e:
            print(f"    warn quantize: {e} — sử dụng FP32 thay thế")

    # ── Verify ────────────────────────────────────────────────────────────────
    print(f"\n[4/4] Verifying ONNX model …")
    try:
        import onnx
        m = onnx.load(target_path)
        print(f"    Opset   : {m.opset_import[0].version}")
        print(f"    Inputs  : {[i.name for i in m.graph.input]}")
        print(f"    Outputs : {[o.name for o in m.graph.output]}")

        import onnxruntime as ort
        import numpy as np

        sess = ort.InferenceSession(target_path, providers=["CPUExecutionProvider"])
        dummy_cpu = tokenizer(
            "Microsoft was founded by Bill Gates.",
            return_tensors="np",
            max_length=64,
            padding="max_length",
            truncation=True,
        )
        feeds = {k: v.astype(np.int64) for k, v in dummy_cpu.items()
                 if k in [i.name for i in sess.get_inputs()]}
        out = sess.run(None, feeds)
        logits = out[0]  # [1, seq, num_labels]
        preds = logits[0].argmax(-1)
        pred_labels = [labels.get(str(i), "O") for i in preds[:8]]
        print(f"    Sample  : {pred_labels}  (first 8 tokens)")
        print(f"    ✓ ONNX Runtime inference OK")
    except Exception as e:
        print(f"    warn verify: {e}")

    size_mb = os.path.getsize(target_path) / 1e6
    quant_str = " (INT8)" if args.quantize else " (FP32)"
    print(f"\n✅  Done! model.onnx = {size_mb:.1f} MB{quant_str}")
    print(f"    Model  : {args.model}")
    print(f"    Output : {os.path.abspath(args.output)}/")
    print(f"""
RAM ước tính khi chạy:
  ONNX Runtime (CPU) : ~{size_mb * 1.5:.0f} MB
  + Harrier CPU      : ~1 500 MB
  Tổng               : ~{size_mb * 1.5 + 1500:.0f} MB

Go code:
    deb, _ := deberta.NewExtractor(deberta.Config{{
        ModelPath:     "{args.output}/model.onnx",
        TokenizerPath: "{args.output}/tokenizer.json",
        LabelsPath:    "{args.output}/labels.json",
        MaxSeqLen:     {args.seq_len},
    }})
""")


if __name__ == "__main__":
    main()
