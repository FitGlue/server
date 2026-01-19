#!/usr/bin/env python3
"""
Build per-handler TypeScript ZIPs for Cloud Functions deployment.

Creates deterministic ZIPs with SOURCE files (Cloud Build compiles):
- Handler's source files (src/, package.json, tsconfig.json)
- shared/ source files
- Root package.json + package-lock.json

Excludes: node_modules, dist, build, coverage (Cloud Build handles these)

Usage: python3 scripts/build_typescript_zips.py
"""
import os
import shutil
import zipfile
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

# Handlers excluded from ZIP generation (not Cloud Functions)
EXCLUDED_DIRS = {'shared', 'admin-cli', 'mcp-server', 'node_modules'}

# Patterns to exclude from ZIPs (same as Terraform archive_file)
EXCLUDE_PATTERNS = {'node_modules', 'dist', 'build', 'coverage', '.DS_Store'}


def get_handler_dirs(ts_src_dir: Path) -> list[str]:
    """Auto-discover handler directories."""
    handlers = []
    for item in sorted(ts_src_dir.iterdir()):
        if item.is_dir() and item.name not in EXCLUDED_DIRS:
            if (item / 'package.json').exists():
                handlers.append(item.name)
    return handlers


def should_exclude(path: Path) -> bool:
    """Check if path should be excluded from ZIP."""
    for part in path.parts:
        if part in EXCLUDE_PATTERNS:
            return True
    return False


def copy_filtered(src: Path, dst: Path):
    """Copy directory excluding node_modules, dist, build, etc."""
    if not src.exists():
        return

    for item in src.iterdir():
        if item.name in EXCLUDE_PATTERNS:
            continue

        dst_path = dst / item.name
        if item.is_file():
            dst_path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(item, dst_path)
        elif item.is_dir():
            copy_filtered(item, dst_path)


def create_handler_zip(handler_name: str, ts_src_dir: Path, output_dir: Path) -> str:
    """Create a deployment zip for a TypeScript Cloud Function handler."""
    print(f"Creating zip for {handler_name}...")

    handler_dir = ts_src_dir / handler_name
    temp_dir = output_dir / f"{handler_name}_temp"
    zip_path = output_dir / f"{handler_name}.zip"

    # Clean temp directory
    if temp_dir.exists():
        shutil.rmtree(temp_dir)
    temp_dir.mkdir(parents=True)

    # Copy handler SOURCE files to handler subdirectory (preserving structure)
    handler_dest = temp_dir / handler_name
    copy_filtered(handler_dir, handler_dest)

    # Copy shared/ source files
    shared_dir = ts_src_dir / 'shared'
    if shared_dir.exists():
        copy_filtered(shared_dir, temp_dir / 'shared')

    # Generate custom package.json for this ZIP with only handler + shared as workspaces
    # This ensures Cloud Build builds shared first, then the handler
    import json

    with open(ts_src_dir / 'package.json', 'r') as f:
        root_pkg = json.load(f)

    # Create ZIP-specific package.json
    # Determine output dir (build or dist) by checking handler's tsconfig
    handler_tsconfig = handler_dir / 'tsconfig.json'
    output_dir = 'build'  # default
    if handler_tsconfig.exists():
        with open(handler_tsconfig, 'r') as f:
            try:
                tsconfig = json.load(f)
                out_dir = tsconfig.get('compilerOptions', {}).get('outDir', 'build')
                output_dir = out_dir.replace('./', '').strip('/')
            except:
                pass
    
    zip_pkg = {
        "private": True,
        "main": "index.js",  # Root entry point for Cloud Functions
        "workspaces": ["shared", handler_name],
        "scripts": {
            "build": "npm run build --workspace=@fitglue/shared && npm run build --workspace=" + handler_name,
            "gcp-build": "npm run build"
        },
        "devDependencies": root_pkg.get("devDependencies", {}),
        "dependencies": root_pkg.get("dependencies", {}),
        "overrides": root_pkg.get("overrides", {})
    }

    with open(temp_dir / 'package.json', 'w') as f:
        json.dump(zip_pkg, f, indent=2)

    # Copy package-lock.json (needed for reproducible installs)
    lock_file = ts_src_dir / 'package-lock.json'
    if lock_file.exists():
        shutil.copy2(lock_file, temp_dir / 'package-lock.json')
    
    # Generate index.js that re-exports all handler exports (Cloud Functions entry point)
    index_js = f"""// Auto-generated entry point for {handler_name}
const handler = require('./{handler_name}/{output_dir}/index');
module.exports = handler;
"""
    with open(temp_dir / 'index.js', 'w') as f:
        f.write(index_js)

    # Create deterministic zip
    with zipfile.ZipFile(zip_path, 'w', zipfile.ZIP_DEFLATED) as zipf:
        all_files = []
        for root, dirs, files in os.walk(temp_dir):
            dirs.sort()  # Deterministic walk order
            for file in sorted(files):
                all_files.append(Path(root) / file)

        for file_path in all_files:
            arcname = str(file_path.relative_to(temp_dir))

            # Create ZipInfo with fixed metadata for deterministic hashing
            zinfo = zipfile.ZipInfo(filename=arcname)
            zinfo.date_time = (1980, 1, 1, 0, 0, 0)  # Fixed timestamp
            zinfo.external_attr = 0o644 << 16  # rw-r--r--
            zinfo.compress_type = zipfile.ZIP_DEFLATED

            with open(file_path, 'rb') as f:
                data = f.read()
            zipf.writestr(zinfo, data)

    # Clean up temp directory
    shutil.rmtree(temp_dir)

    print(f"  Created {zip_path}")
    return str(zip_path)


def main():
    script_dir = Path(__file__).parent
    ts_src_dir = script_dir.parent / "src" / "typescript"
    output_dir = Path("/tmp/fitglue-function-zips")

    # Discover handlers
    handlers = get_handler_dirs(ts_src_dir)
    print(f"Discovered {len(handlers)} handlers: {', '.join(handlers)}")

    # Create output directory (don't clean - Go zips may already be there)
    output_dir.mkdir(parents=True, exist_ok=True)

    # Create zips in parallel
    max_workers = min(8, len(handlers))  # Cap at 8 threads
    print(f"Creating ZIPs with {max_workers} parallel workers...")

    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = {
            executor.submit(create_handler_zip, handler, ts_src_dir, output_dir): handler
            for handler in handlers
        }

        for future in as_completed(futures):
            handler = futures[future]
            try:
                future.result()
            except Exception as e:
                print(f"  ERROR creating {handler}: {e}")

    print(f"\nAll {len(handlers)} TypeScript function zips created in {output_dir}")


if __name__ == "__main__":
    main()
