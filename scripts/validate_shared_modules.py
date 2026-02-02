#!/usr/bin/env python3
"""
Validate that shared_modules.json is in sync with the actual shared/ directory structure.

This script is meant to be run in CI to catch drift between:
1. Module definitions and actual paths
2. New directories that need module definitions
3. Stale paths that no longer exist

Usage: python3 scripts/validate_shared_modules.py
Exit codes:
  0 = valid
  1 = validation errors found
"""

import json
import sys
from pathlib import Path
from typing import Set

# Directories/files to ignore when scanning shared/src/
IGNORE_PATTERNS = {
    '__pycache__',
    'node_modules',
    'dist',
    'build',
    '.DS_Store',
    '*.test.ts',
    '*.spec.ts',
}

# Files that are expected at the root but don't need module definitions
ROOT_CONFIG_FILES = {
    'index.ts',
    'config.ts',
    'interfaces.ts',
    'constants.ts',
}


def load_modules_config(script_dir: Path) -> dict:
    """Load the shared modules configuration."""
    config_path = script_dir / 'shared_modules.json'
    with open(config_path, 'r') as f:
        return json.load(f)


def get_defined_paths(modules_config: dict) -> Set[str]:
    """Extract all paths defined in modules config."""
    paths = set()
    for module in modules_config.get('modules', {}).values():
        for path in module.get('paths', []):
            paths.add(path)
    return paths


def get_actual_paths(shared_src_dir: Path) -> Set[str]:
    """
    Scan shared/src/ and return paths that should have module definitions.

    Returns paths like 'src/framework', 'src/integrations/strava', etc.
    """
    paths = set()

    if not shared_src_dir.exists():
        return paths

    def should_ignore(name: str) -> bool:
        if name.startswith('.'):
            return True
        if name in IGNORE_PATTERNS:
            return True
        if name.endswith('.test.ts') or name.endswith('.spec.ts'):
            return True
        return False

    def scan_dir(dir_path: Path, rel_prefix: str):
        """Recursively scan directory and add significant paths."""
        for item in sorted(dir_path.iterdir()):
            if should_ignore(item.name):
                continue

            rel_path = f"{rel_prefix}/{item.name}" if rel_prefix else item.name

            if item.is_dir():
                # Add directory as a path
                paths.add(f"src/{rel_path}")
                # Don't recurse into integration subdirs - they're leaf modules
                if 'integrations/' not in rel_path or rel_path.count('/') < 2:
                    scan_dir(item, rel_path)
            elif item.is_file() and item.suffix == '.ts':
                # Only add standalone .ts files at src/ root level
                if '/' not in rel_path and item.name not in ROOT_CONFIG_FILES:
                    paths.add(f"src/{rel_path}")

    scan_dir(shared_src_dir, '')
    return paths


def get_top_level_modules(shared_src_dir: Path) -> Set[str]:
    """Get top-level directories that should be modules."""
    modules = set()

    if not shared_src_dir.exists():
        return modules

    for item in shared_src_dir.iterdir():
        if item.is_dir() and not item.name.startswith('.'):
            modules.add(item.name)

    return modules


def validate_paths_exist(modules_config: dict, shared_dir: Path) -> list[str]:
    """Check that all paths in config actually exist."""
    errors = []

    for mod_name, mod_config in modules_config.get('modules', {}).items():
        for path in mod_config.get('paths', []):
            full_path = shared_dir / path
            if not full_path.exists():
                errors.append(f"Module '{mod_name}' references non-existent path: {path}")

    return errors


def validate_dependencies_exist(modules_config: dict) -> list[str]:
    """Check that all dependency references are valid module names."""
    errors = []
    module_names = set(modules_config.get('modules', {}).keys())

    for mod_name, mod_config in modules_config.get('modules', {}).items():
        for dep in mod_config.get('depends_on', []):
            if dep not in module_names:
                errors.append(f"Module '{mod_name}' depends on unknown module: {dep}")

    return errors


def validate_import_patterns(modules_config: dict) -> list[str]:
    """Check that import patterns reference valid modules."""
    errors = []
    module_names = set(modules_config.get('modules', {}).keys())

    for pattern, modules in modules_config.get('import_patterns', {}).items():
        if isinstance(modules, str):
            modules = [modules]
        for mod in modules:
            if mod not in module_names:
                errors.append(f"Import pattern '{pattern}' references unknown module: {mod}")

    return errors


def validate_barrel_exports(modules_config: dict, shared_dir: Path) -> list[str]:
    """
    Check that package.json subpath exports have corresponding barrel modules.
    
    For each export like "./domain", we need to ensure src/domain/index.ts is
    included in some module's paths. Otherwise, the pruned ZIP will be missing
    the barrel file and TypeScript will fail to resolve the import.
    """
    errors = []
    
    # Load package.json
    package_json_path = shared_dir / 'package.json'
    if not package_json_path.exists():
        return errors
    
    with open(package_json_path, 'r') as f:
        package_json = json.load(f)
    
    exports = package_json.get('exports', {})
    defined_paths = get_defined_paths(modules_config)
    
    # Subpaths that use dist/* wildcard pattern - these don't need barrel coverage
    wildcard_patterns = [k for k in exports.keys() if '*' in k]
    
    for subpath, export_config in exports.items():
        # Skip root export and wildcard patterns
        if subpath == '.' or '*' in subpath:
            continue
        
        # Extract the subpath (e.g., "./domain" -> "domain")
        clean_subpath = subpath.lstrip('./')
        
        # Determine expected barrel file path
        barrel_path = f"src/{clean_subpath}/index.ts"
        
        # Check if this barrel file exists on disk
        barrel_file = shared_dir / barrel_path
        if not barrel_file.exists():
            # It might be a direct file export, not a directory with barrel
            # Check if export points to a specific file
            if isinstance(export_config, dict):
                types_path = export_config.get('types', '')
                if types_path and not types_path.endswith('/index.d.ts'):
                    continue  # Direct file export, not a barrel
            continue
        
        # Check if this barrel path is included in any module
        barrel_in_modules = barrel_path in defined_paths
        
        # Also check if the directory is covered (some modules define the dir, not the file)
        dir_path = f"src/{clean_subpath}"
        dir_in_modules = dir_path in defined_paths
        
        if not barrel_in_modules and not dir_in_modules:
            errors.append(
                f"Package.json export '{subpath}' needs barrel module: "
                f"'{barrel_path}' is not included in any module's paths"
            )
    
    return errors


def find_uncovered_directories(modules_config: dict, shared_src_dir: Path) -> list[str]:
    """Find directories in shared/src/ that don't have module definitions."""
    warnings = []
    defined_paths = get_defined_paths(modules_config)

    # Check top-level directories
    for item in sorted(shared_src_dir.iterdir()):
        if item.is_dir() and not item.name.startswith('.'):
            dir_path = f"src/{item.name}"

            # Check if this directory or any of its contents is covered
            is_covered = any(
                p == dir_path or p.startswith(f"{dir_path}/")
                for p in defined_paths
            )

            if not is_covered:
                warnings.append(f"Directory '{dir_path}' has no module definition")

            # Check subdirectories (one level deep for integrations, etc.)
            if item.name == 'integrations':
                for subitem in sorted(item.iterdir()):
                    if subitem.is_dir() and not subitem.name.startswith('.'):
                        subdir_path = f"src/{item.name}/{subitem.name}"
                        is_sub_covered = any(
                            p == subdir_path or p.startswith(f"{subdir_path}/")
                            for p in defined_paths
                        )
                        if not is_sub_covered:
                            warnings.append(f"Integration '{subdir_path}' has no module definition")

    return warnings


def main():
    script_dir = Path(__file__).parent
    shared_dir = script_dir.parent / "src" / "typescript" / "shared"
    shared_src_dir = shared_dir / "src"

    print("Validating shared_modules.json...")
    print(f"  Shared directory: {shared_dir}")
    print()

    # Load config
    try:
        modules_config = load_modules_config(script_dir)
        print(f"  Loaded {len(modules_config.get('modules', {}))} module definitions")
        print(f"  Loaded {len(modules_config.get('import_patterns', {}))} import patterns")
    except FileNotFoundError:
        print("ERROR: shared_modules.json not found!")
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"ERROR: Invalid JSON in shared_modules.json: {e}")
        sys.exit(1)

    print()

    # Run validations
    all_errors = []
    all_warnings = []

    # Check paths exist
    errors = validate_paths_exist(modules_config, shared_dir)
    all_errors.extend(errors)

    # Check dependencies are valid
    errors = validate_dependencies_exist(modules_config)
    all_errors.extend(errors)

    # Check import patterns reference valid modules
    errors = validate_import_patterns(modules_config)
    all_errors.extend(errors)

    # Check package.json exports have corresponding barrel modules
    errors = validate_barrel_exports(modules_config, shared_dir)
    all_errors.extend(errors)

    # Find uncovered directories (warnings, not errors)
    warnings = find_uncovered_directories(modules_config, shared_src_dir)
    all_warnings.extend(warnings)

    # Report results
    if all_errors:
        print("ERRORS:")
        for error in all_errors:
            print(f"  ❌ {error}")
        print()

    if all_warnings:
        print("WARNINGS (new code may need module definitions):")
        for warning in all_warnings:
            print(f"  ⚠️  {warning}")
        print()

    if all_errors:
        print(f"FAILED: {len(all_errors)} error(s) found")
        print()
        print("To fix:")
        print("  1. Update server/scripts/shared_modules.json")
        print("  2. Add missing paths or remove stale references")
        print("  3. Ensure all dependencies reference valid module names")
        sys.exit(1)

    if all_warnings:
        print(f"PASSED with {len(all_warnings)} warning(s)")
        print("Consider adding module definitions for new directories.")
    else:
        print("PASSED: All validations successful!")

    sys.exit(0)


if __name__ == "__main__":
    main()
