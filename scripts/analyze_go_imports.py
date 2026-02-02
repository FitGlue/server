#!/usr/bin/env python3
"""
Analyze Go function imports to determine which pkg/ subdirectories each function needs.

This enables smart pruning in build_function_zips.py - only copying the packages
that each function actually imports (directly or transitively).

Usage:
    python3 analyze_go_imports.py                    # Show all functions
    python3 analyze_go_imports.py router             # Analyze specific function
    python3 analyze_go_imports.py --json             # Output as JSON for scripting
"""

import subprocess
import json
import sys
from pathlib import Path
from typing import Set, Dict, List

# Go source directory
SCRIPT_DIR = Path(__file__).parent
GO_SRC_DIR = SCRIPT_DIR.parent / "src" / "go"

# Module path prefix for internal packages
MODULE_PREFIX = "github.com/fitglue/server/src/go/pkg"

# List of Go functions
GO_FUNCTIONS = [
    "router",
    "enricher",
    "pipeline-splitter",
    "strava-uploader",
    "mock-uploader",
    "parkrun-results-source",
    "showcase-uploader",
    "fit-parser-handler",
    "hevy-uploader",
    "trainingpeaks-uploader",
    "intervals-uploader",
    "googlesheets-uploader",
]


def get_pkg_dependencies(function_name: str) -> Set[str]:
    """
    Get all pkg/ subdirectories that a function depends on (directly or transitively).

    Uses `go list -deps` to get the full dependency tree, then filters to just
    internal pkg/ packages.

    Returns:
        Set of package paths relative to pkg/, e.g. {"bootstrap", "types/pb", "infrastructure/pubsub"}
    """
    function_path = f"./functions/{function_name}/..."

    try:
        result = subprocess.run(
            ["go", "list", "-f", "{{.ImportPath}}", "-deps", function_path],
            cwd=GO_SRC_DIR,
            capture_output=True,
            text=True,
            check=True
        )
    except subprocess.CalledProcessError as e:
        print(f"Error analyzing {function_name}: {e.stderr}", file=sys.stderr)
        return set()

    pkg_deps = set()
    for line in result.stdout.strip().split("\n"):
        if line.startswith(MODULE_PREFIX):
            # Extract the path relative to pkg/
            relative_path = line[len(MODULE_PREFIX):].lstrip("/")
            if relative_path:  # Skip the root pkg itself
                pkg_deps.add(relative_path)

    return pkg_deps


def get_pkg_directories(pkg_deps: Set[str]) -> Set[str]:
    """
    Convert package import paths to directory paths that need to be copied.

    For nested packages like "infrastructure/pubsub", we need to ensure the
    parent directory structure exists, but we only need to copy the specific
    subdirectory.

    Returns:
        Set of directory paths relative to pkg/ that need to be copied
    """
    directories = set()

    for dep in pkg_deps:
        # Add the package directory itself
        directories.add(dep)

        # For nested packages, we might also need parent files
        # e.g., pkg/infrastructure/ might have common files
        parts = dep.split("/")
        for i in range(1, len(parts)):
            parent = "/".join(parts[:i])
            directories.add(parent)

    return directories


def analyze_all_functions() -> Dict[str, Set[str]]:
    """Analyze all Go functions and return their pkg dependencies."""
    results = {}
    for fn in GO_FUNCTIONS:
        results[fn] = get_pkg_dependencies(fn)
    return results


def print_analysis(results: Dict[str, Set[str]], json_output: bool = False):
    """Print analysis results in human-readable or JSON format."""
    if json_output:
        # Convert sets to sorted lists for JSON
        json_results = {fn: sorted(deps) for fn, deps in results.items()}
        print(json.dumps(json_results, indent=2))
        return

    print(f"Analyzing {len(GO_FUNCTIONS)} Go functions...\n")

    # Find all unique packages
    all_packages = set()
    for deps in results.values():
        all_packages.update(deps)

    # Print per-function analysis
    for fn in sorted(results.keys()):
        deps = results[fn]
        print(f"{fn}: {len(deps)} packages")

    print(f"\n=== Summary ===\n")
    print(f"Total unique pkg/ packages used: {len(all_packages)}")

    # Count usage per package
    pkg_usage: Dict[str, int] = {}
    for deps in results.values():
        for pkg in deps:
            pkg_usage[pkg] = pkg_usage.get(pkg, 0) + 1

    print(f"\n=== Package Usage (most to least common) ===\n")
    for pkg, count in sorted(pkg_usage.items(), key=lambda x: (-x[1], x[0])):
        marker = "🔥" if count == len(GO_FUNCTIONS) else "  "
        print(f"{marker} {pkg}: {count}/{len(GO_FUNCTIONS)} functions")


def main():
    args = sys.argv[1:]

    json_output = "--json" in args
    args = [a for a in args if a != "--json"]

    if args:
        # Analyze specific function(s)
        results = {}
        for fn in args:
            if fn in GO_FUNCTIONS:
                results[fn] = get_pkg_dependencies(fn)
            else:
                print(f"Unknown function: {fn}", file=sys.stderr)
                print(f"Available: {', '.join(GO_FUNCTIONS)}", file=sys.stderr)
                sys.exit(1)
    else:
        # Analyze all functions
        results = analyze_all_functions()

    print_analysis(results, json_output)


if __name__ == "__main__":
    main()
