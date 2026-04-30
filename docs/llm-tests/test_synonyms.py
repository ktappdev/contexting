#!/usr/bin/env python3
"""
Test LLM synonym generation for contexting.

Usage:
    # Local endpoint, 0.8b, 4 parallel batches of 15, 4 synonyms
    python3 test_synonyms.py --model qwen3.5-0.8b --batch-size 15 --parallel 4 --synonyms 4

    # Flex range: accept 4-8 synonyms
    python3 test_synonyms.py --model qwen3.5-0.8b --batch-size 15 --parallel 4 --synonyms-min 4 --synonyms-max 8

    # OpenRouter with API key
    python3 test_synonyms.py --model qwen3.5-4b --endpoint https://openrouter.ai/api/v1/chat/completions --api-key $KEY
"""

import json
import subprocess
import argparse
import sys
import os
import time
import concurrent.futures

DEFAULT_ENDPOINT = "https://llama.kentaylor.dev/v1/chat/completions"
DEFAULT_CONTEXT_JSON = os.path.expanduser(
    "/Users/kentaylor/developer/vault/vaultgy/context.json"
)

SYSTEM_PROMPT_FIXED = (
    'You are a helpful assistant. For each folder or file name in the list, '
    'generate exactly {synonyms} plausible alternative words or short phrases '
    'a developer might use when searching for that file in a codebase. '
    'Return ONLY a valid JSON object where each key is an exact filename from '
    'the input list and each value is an array of {synonyms} synonym strings. '
    'Example: {{"auth.go": ["login", "authentication", "session"]}}. '
    'No markdown, no prose, no extra text.'
)

SYSTEM_PROMPT_FLEX = (
    'You are a helpful assistant. For each folder or file name in the list, '
    'generate {min_syn} to {max_syn} plausible alternative words or short phrases '
    'a developer might use when searching for that file in a codebase. '
    'Aim for as many as you can, but at least {min_syn} and no more than {max_syn}. '
    'Return ONLY a valid JSON object where each key is an exact filename from '
    'the input list and each value is an array of synonym strings. '
    'Example: {{"auth.go": ["login", "authentication", "session"]}}. '
    'No markdown, no prose, no extra text.'
)


def collect_names(node, names=None):
    if names is None:
        names = set()
    for key, child in node.get("children", {}).items():
        names.add(key)
        collect_names(child, names)
    return names


def load_names(context_json, limit):
    with open(context_json) as f:
        d = json.load(f)
    names = sorted(collect_names(d["tree"]))
    return names[:limit]


def fetch_batch(names, model, system_prompt, endpoint, api_key, timeout=180, no_reasoning=False):
    """Fetch synonyms for a single batch. Returns result dict."""
    headers = ["-H", "Content-Type: application/json"]
    if api_key:
        headers += ["-H", f"Authorization: Bearer {api_key}"]

    msg = {
        "model": model,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": "File and folder names:\n" + "\n".join(names)},
        ],
        "temperature": 0.3,
    }
    if no_reasoning:
        msg["reasoning"] = {"exclude": True}
    payload = json.dumps(msg)

    try:
        result = subprocess.run(
            ["curl", "-s", "--max-time", str(timeout), endpoint] + headers + ["-d", payload],
            capture_output=True,
            text=True,
            timeout=timeout + 5,
        )
        d = json.loads(result.stdout)
    except json.JSONDecodeError:
        return {
            "valid": False,
            "error": f"API returned non-JSON: {result.stdout[:100]}",
            "tokens": 0,
        }
    except subprocess.TimeoutExpired:
        return {"valid": False, "error": "Request timed out", "tokens": 0}

    if "error" in d:
        return {
            "valid": False,
            "error": d["error"].get("message", str(d["error"]))[:100],
            "tokens": 0,
        }

    content = d["choices"][0]["message"]["content"]
    usage = d["usage"]

    decoder = json.JSONDecoder()
    try:
        obj, _ = decoder.raw_decode(content)
        return {"valid": True, "obj": obj, "tokens": usage.get("total_tokens", 0)}
    except Exception as e:
        return {"valid": False, "error": str(e)[:80], "tokens": usage.get("total_tokens", 0)}


def run_parallel(names, batch_size, parallel, model, system_prompt, endpoint, api_key, timeout, no_reasoning=False):
    """Split names into batches, fetch in parallel, merge results."""
    batches = [names[i : i + batch_size] for i in range(0, len(names), batch_size)]

    start = time.time()
    with concurrent.futures.ThreadPoolExecutor(max_workers=parallel) as pool:
        futures = [
            pool.submit(fetch_batch, b, model, system_prompt, endpoint, api_key, timeout, no_reasoning)
            for b in batches
        ]
        results = [f.result() for f in futures]
    wall = time.time() - start

    merged = {}
    for r in results:
        if r["valid"]:
            merged.update(r["obj"])

    all_ok = all(r["valid"] for r in results)
    total_tokens = sum(r["tokens"] for r in results)

    if all_ok:
        counts = [len(v) for v in merged.values() if isinstance(v, list)]
        return {
            "valid": True,
            "names_returned": len(merged),
            "synonyms_min": min(counts) if counts else 0,
            "synonyms_max": max(counts) if counts else 0,
            "synonyms_avg": round(sum(counts) / len(counts), 1) if counts else 0,
            "tokens": total_tokens,
            "wall_s": round(wall, 1),
            "missing": [],
            "extra": [],
        }
    else:
        errors = [
            f"{r.get('error', 'unknown')}" for r in results if not r["valid"]
        ]
        return {
            "valid": False,
            "error": "; ".join(errors)[:150],
            "tokens": total_tokens,
            "wall_s": round(wall, 1),
        }


def main():
    parser = argparse.ArgumentParser(description="Test LLM synonym generation")
    parser.add_argument("--model", required=True, help="Model name (e.g. qwen3.5-0.8b)")
    parser.add_argument("--batch-size", type=int, default=60, help="Names per batch")
    parser.add_argument("--parallel", type=int, default=1, help="Parallel requests")
    parser.add_argument("--synonyms", type=int, default=4, help="Exact synonyms per name")
    parser.add_argument("--synonyms-min", type=int, help="Min synonyms (flex mode)")
    parser.add_argument("--synonyms-max", type=int, help="Max synonyms (flex mode)")
    parser.add_argument("--runs", type=int, default=5, help="Number of test runs")
    parser.add_argument("--endpoint", default=DEFAULT_ENDPOINT, help="LLM API endpoint")
    parser.add_argument("--api-key", default="", help="API key (empty for local)")
    parser.add_argument("--context-json", default=DEFAULT_CONTEXT_JSON, help="Path to context.json")
    parser.add_argument("--timeout", type=int, default=180, help="Per-request timeout (seconds)")
    parser.add_argument("--no-reasoning", action="store_true", help="Disable reasoning/thinking tokens (OpenRouter)")
    parser.add_argument("--output", help="Output markdown file (auto-generated if omitted)")
    args = parser.parse_args()

    # Build system prompt
    if args.synonyms_min and args.synonyms_max:
        system_prompt = SYSTEM_PROMPT_FLEX.format(
            min_syn=args.synonyms_min, max_syn=args.synonyms_max
        )
        syn_desc = f"{args.synonyms_min}-{args.synonyms_max} (flex)"
    else:
        system_prompt = SYSTEM_PROMPT_FIXED.format(synonyms=args.synonyms)
        syn_desc = str(args.synonyms)

    total_names = args.batch_size * args.parallel
    names = load_names(args.context_json, total_names)

    print(f"Model: {args.model}")
    print(f"Endpoint: {args.endpoint}")
    print(f"Config: {args.parallel}x{args.batch_size} parallel | {syn_desc} synonyms | {args.runs} runs")
    print(f"Names: {len(names)} total ({args.batch_size} per batch x {args.parallel} batches)")
    print()

    results = []
    for i in range(args.runs):
        print(f"Run {i+1}/{args.runs}...", end=" ", flush=True)
        r = run_parallel(
            names, args.batch_size, args.parallel,
            args.model, system_prompt, args.endpoint, args.api_key, args.timeout,
            no_reasoning=args.no_reasoning,
        )
        results.append(r)
        if r["valid"]:
            print(
                f"VALID | {r['names_returned']}/{len(names)} names | "
                f"syn={r['synonyms_min']}-{r['synonyms_max']} avg={r['synonyms_avg']} | "
                f"tokens={r['tokens']} | {r['wall_s']}s"
            )
        else:
            print(f"FAILED | {r['error'][:60]} | tokens={r['tokens']} | {r['wall_s']}s")

    # Summary
    valid_count = sum(1 for r in results if r["valid"])
    print(f"\n{'='*60}")
    print(f"Summary: {valid_count}/{args.runs} valid JSON")
    if valid_count > 0:
        valid_results = [r for r in results if r["valid"]]
        avg_names = sum(r["names_returned"] for r in valid_results) / valid_count
        avg_tokens = sum(r["tokens"] for r in valid_results) / valid_count
        avg_wall = sum(r["wall_s"] for r in valid_results) / valid_count
        avg_syn = sum(r["synonyms_avg"] for r in valid_results) / valid_count
        print(f"Avg names returned: {avg_names:.1f}/{len(names)}")
        print(f"Avg synonyms: {avg_syn:.1f}")
        print(f"Avg tokens: {avg_tokens:.0f}")
        print(f"Avg wall time: {avg_wall:.1f}s")

    # Write markdown
    tag = args.output or f"results/{args.model}"
    if args.synonyms_min:
        tag += f"-flex{args.synonyms_min}-{args.synonyms_max}"
    if args.parallel > 1:
        tag += f"-{args.parallel}x{args.batch_size}"
    outpath = tag + ".md"

    os.makedirs(os.path.dirname(outpath), exist_ok=True)
    with open(outpath, "w") as f:
        f.write(f"# {args.model} Results\n\n")
        f.write(f"- Endpoint: {args.endpoint}\n")
        f.write(f"- Config: {args.parallel}x{args.batch_size} parallel | {syn_desc} synonyms\n")
        f.write(f"- Runs: {args.runs}\n")
        f.write(f"- Source: {args.context_json}\n\n")
        f.write(f"## Summary\n\n")
        f.write(f"- Valid JSON: **{valid_count}/{args.runs}**\n")
        if valid_count > 0:
            f.write(f"- Avg names returned: {avg_names:.1f}/{len(names)}\n")
            f.write(f"- Avg synonyms: {avg_syn:.1f}\n")
            f.write(f"- Avg tokens: {avg_tokens:.0f}\n")
            f.write(f"- Avg wall time: {avg_wall:.1f}s\n")
        f.write(f"\n## Runs\n\n")
        f.write("| Run | Valid | Names | Synonyms | Tokens | Time |\n")
        f.write("|-----|-------|-------|----------|--------|------|\n")
        for i, r in enumerate(results):
            if r["valid"]:
                f.write(
                    f"| {i+1} | YES | {r['names_returned']}/{len(names)} | "
                    f"{r['synonyms_min']}-{r['synonyms_max']} avg={r['synonyms_avg']} | "
                    f"{r['tokens']} | {r['wall_s']}s |\n"
                )
            else:
                f.write(f"| {i+1} | NO | — | — | {r['tokens']} | {r['wall_s']}s |\n")
        # Invalid details
        invalid = [(i, r) for i, r in enumerate(results) if not r["valid"]]
        if invalid:
            f.write(f"\n## Failures\n\n")
            for i, r in invalid:
                f.write(f"- Run {i+1}: `{r['error']}`\n")

    print(f"\nResults written to {outpath}")


if __name__ == "__main__":
    main()
