#!/usr/bin/env python3
"""
INT8 dynamic quantization for Harrier & DeBERTa ONNX (CPU-friendly).

FP16 full-graph conversion is intentionally not offered: onnxconverter_common's
convert_float_to_float16 yields graphs ONNX Runtime rejects for these transformer
exports (dtype mismatches on Cast / Add). Use FP32 originals or INT8 here.

Usage:
    python scripts/convert_onnx_precision.py int8 \\
        data/harrier/model.onnx data/harrier-q/model.onnx

Copy tokenizer.json (and labels.json for NER) into the output directory yourself,
or run from a script that copies sidecars (see Makefile quantize-* targets).
"""
from __future__ import annotations

import argparse
import os
import sys


def main() -> None:
    p = argparse.ArgumentParser(description="Dynamic-quantize ONNX to INT8.")
    p.add_argument("input_onnx")
    p.add_argument("output_onnx")
    args = p.parse_args()

    inp = args.input_onnx
    outp = args.output_onnx

    if not os.path.isfile(inp):
        print(f"error: input not found: {inp}", file=sys.stderr)
        sys.exit(1)
    os.makedirs(os.path.dirname(outp) or ".", exist_ok=True)

    try:
        from onnxruntime.quantization import QuantType, quantize_dynamic
    except ImportError as e:
        print("Install: pip install onnxruntime", file=sys.stderr)
        raise SystemExit(1) from e
    quantize_dynamic(inp, outp, weight_type=QuantType.QInt8)
    print(f"INT8 → {outp} ({os.path.getsize(outp) / 1e6:.1f} MB)")


if __name__ == "__main__":
    main()
