#!/usr/bin/env python3
"""
Build deployment zips for Go Cloud Functions with smart pruning.

Smart pruning analyzes each function's imports and only includes the pkg/
subdirectories that are actually used. This reduces zip sizes and enables
better CI/CD caching - changes to unused packages won't trigger rebuilds.

Usage:
    python3 build_function_zips.py                  # Build all functions
    python3 build_function_zips.py --no-prune       # Disable pruning (include all pkg/)
"""
import os
import shutil
import subprocess
import sys
import zipfile
from pathlib import Path
from typing import Set

# Module path prefix for internal packages
MODULE_PREFIX = "github.com/fitglue/server/src/go/pkg"


def get_pkg_dependencies(function_name: str, src_dir: Path) -> Set[str]:
    """
    Get all pkg/ subdirectories that a function depends on (directly or transitively).
    
    Uses `go list -deps` to get the full dependency tree, then filters to just
    internal pkg/ packages.
    
    Returns:
        Set of package paths relative to pkg/, e.g. {"bootstrap", "types/pb"}
    """
    function_path = f"./functions/{function_name}/..."
    
    try:
        result = subprocess.run(
            ["go", "list", "-f", "{{.ImportPath}}", "-deps", function_path],
            cwd=src_dir,
            capture_output=True,
            text=True,
            check=True
        )
    except subprocess.CalledProcessError as e:
        print(f"Warning: Could not analyze {function_name}, including all pkg/: {e.stderr}")
        return None  # Signal to include everything
    
    pkg_deps = set()
    for line in result.stdout.strip().split("\n"):
        if line.startswith(MODULE_PREFIX):
            # Extract the path relative to pkg/
            relative_path = line[len(MODULE_PREFIX):].lstrip("/")
            if relative_path:  # Skip the root pkg itself
                pkg_deps.add(relative_path)
    
    return pkg_deps


def copy_pruned_pkg(src_pkg: Path, dest_pkg: Path, needed_packages: Set[str]):
    """
    Copy only the needed packages from pkg/ to the destination.
    
    Handles nested packages correctly - if we need "infrastructure/pubsub",
    we copy the entire infrastructure/pubsub/ directory.
    """
    # Also need to copy root-level files in pkg/ (constants.go, interfaces.go)
    for item in src_pkg.iterdir():
        if item.is_file() and item.suffix == ".go" and not item.name.endswith("_test.go"):
            dest_pkg.mkdir(parents=True, exist_ok=True)
            shutil.copy2(item, dest_pkg / item.name)
    
    # Copy needed package directories
    for pkg_path in needed_packages:
        src_path = src_pkg / pkg_path
        dest_path = dest_pkg / pkg_path
        
        if src_path.exists():
            if src_path.is_dir():
                shutil.copytree(
                    src_path,
                    dest_path,
                    ignore=shutil.ignore_patterns('*_test.go'),
                    dirs_exist_ok=True
                )
            elif src_path.is_file():
                dest_path.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src_path, dest_path)


def create_function_zip(function_name: str, src_dir: Path, output_dir: Path, prune: bool = True):
    """Create a deployment zip for a Go Cloud Function"""
    
    # Analyze dependencies if pruning is enabled
    needed_packages = None
    if prune:
        needed_packages = get_pkg_dependencies(function_name, src_dir)
        if needed_packages is not None:
            print(f"Creating zip for {function_name} ({len(needed_packages)} pkg modules)...")
        else:
            print(f"Creating zip for {function_name} (all pkg - analysis failed)...")
    else:
        print(f"Creating zip for {function_name} (all pkg - pruning disabled)...")

    function_dir = src_dir / "functions" / function_name
    temp_dir = output_dir / f"{function_name}_temp"
    zip_path = output_dir / f"{function_name}.zip"

    # Clean temp directory
    if temp_dir.exists():
        shutil.rmtree(temp_dir)
    temp_dir.mkdir(parents=True)

    # Copy function .go files to ROOT (excluding test files and cmd)
    for go_file in function_dir.glob("*.go"):
        # Skip test files, main.go, and anything in cmd
        if go_file.name.endswith("_test.go") or go_file.name == "main.go" or "cmd" in str(go_file):
            continue
        shutil.copy2(go_file, temp_dir / go_file.name)

    # Copy function subdirectories (like providers/) to preserving the import path structure
    # Import paths are relative to the module root (github.com/fitglue/server/src/go),
    # so providers/ needs to be at functions/{function_name}/providers/ in the ZIP
    for subdir in function_dir.iterdir():
        if subdir.is_dir() and subdir.name not in ["cmd", "__pycache__"]:
            target_path = temp_dir / "functions" / function_name / subdir.name
            shutil.copytree(
                subdir,
                target_path,
                ignore=shutil.ignore_patterns('*_test.go', 'cmd')
            )

    # Copy shared pkg directory - either pruned or full
    shared_pkg = src_dir / "pkg"
    if shared_pkg.exists():
        if needed_packages is not None:
            # Smart pruning: only copy needed packages
            copy_pruned_pkg(shared_pkg, temp_dir / "pkg", needed_packages)
        else:
            # Full copy (pruning disabled or analysis failed)
            shutil.copytree(shared_pkg, temp_dir / "pkg", ignore=shutil.ignore_patterns('*_test.go'))

    # Copy go.mod and go.sum to root
    shutil.copy2(src_dir / "go.mod", temp_dir / "go.mod")
    shutil.copy2(src_dir / "go.sum", temp_dir / "go.sum")

    # Create zip deterministically
    with zipfile.ZipFile(zip_path, 'w', zipfile.ZIP_DEFLATED) as zipf:
        # Walk and collect all files first to sort them
        all_files = []
        for root, dirs, files in os.walk(temp_dir):
            dirs[:] = [d for d in dirs if d != 'cmd'] # Skip cmd dirs in walk
            dirs.sort() # Sort directories in place for deterministic walk

            for file in sorted(files): # Sort files
                all_files.append(Path(root) / file)

        for file_path in all_files:
            arcname = str(file_path.relative_to(temp_dir))

            # Create a ZipInfo object manually with fully controlled metadata
            # This ensures deterministic zips regardless of file system metadata
            zinfo = zipfile.ZipInfo(filename=arcname)

            # Set fixed timestamp (1980-01-01 00:00:00) for deterministic hashing
            zinfo.date_time = (1980, 1, 1, 0, 0, 0)

            # Set fixed Unix permissions (0644 = rw-r--r--)
            # Shift left by 16 bits to place in the Unix external_attr field
            zinfo.external_attr = 0o644 << 16

            # Set compression type
            zinfo.compress_type = zipfile.ZIP_DEFLATED

            # Read file data to write via writestr
            with open(file_path, 'rb') as f:
                data = f.read()
            zipf.writestr(zinfo, data)

    # Clean up temp directory
    shutil.rmtree(temp_dir)

    print(f"Created {zip_path}")
    return str(zip_path)

def main():
    # Parse arguments
    prune = "--no-prune" not in sys.argv
    
    script_dir = Path(__file__).parent
    src_dir = script_dir.parent / "src" / "go"
    output_dir = Path("/tmp/fitglue-function-zips")

    # Clean and create output directory
    if output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True)

    if not prune:
        print("Smart pruning DISABLED - including all pkg/ in each zip\n")
    else:
        print("Smart pruning ENABLED - analyzing dependencies per function\n")

    # Create zips for each function
    functions = [
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
    
    for function_name in functions:
        create_function_zip(function_name, src_dir, output_dir, prune=prune)

    print(f"\nAll function zips created in {output_dir}")

if __name__ == "__main__":
    main()
