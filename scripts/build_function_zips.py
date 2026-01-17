#!/usr/bin/env python3
import os
import shutil
import zipfile
from pathlib import Path

def create_function_zip(function_name, src_dir, output_dir):
    """Create a deployment zip for a Go Cloud Function"""
    print(f"Creating zip for {function_name}...")

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

    # Copy shared pkg directory (excluding test files)
    shared_pkg = src_dir / "pkg"
    if shared_pkg.exists():
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
    script_dir = Path(__file__).parent
    src_dir = script_dir.parent / "src" / "go"
    output_dir = Path("/tmp/fitglue-function-zips")

    # Clean and create output directory
    if output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True)

    # Create zips for each function
    for function_name in ["router", "enricher", "strava-uploader", "mock-uploader", "parkrun-results-source"]:
        create_function_zip(function_name, src_dir, output_dir)


    print(f"All function zips created in {output_dir}")

if __name__ == "__main__":
    main()
