#!/usr/bin/env python3
"""
Analyze TypeScript imports to determine required shared modules.

This module provides functions to:
1. Extract @fitglue/shared imports from TypeScript files
2. Extract specific symbols imported from the barrel
3. Map imports/symbols to module IDs defined in shared_modules.json
4. Resolve transitive dependencies between modules
"""

import re
import json
from pathlib import Path
from typing import Set, Dict, Any, Tuple


def load_modules_config(script_dir: Path | None = None) -> Dict[str, Any]:
    """Load the shared modules configuration."""
    if script_dir is None:
        script_dir = Path(__file__).parent
    config_path = script_dir / 'shared_modules.json'
    with open(config_path, 'r') as f:
        return json.load(f)


def extract_imports(file_path: Path) -> Tuple[Set[str], Set[str]]:
    """
    Extract all @fitglue/shared imports from a TypeScript file.

    Returns:
    - Set of import paths (e.g., '@fitglue/shared', '@fitglue/shared/dist/types/pb/user')
    - Set of symbols imported from the barrel '@fitglue/shared'

    Handles:
    - import { ... } from '@fitglue/shared'
    - import { ... } from '@fitglue/shared/dist/...'
    - import * as x from '@fitglue/shared'
    - import type { ... } from '@fitglue/shared'
    """
    try:
        content = file_path.read_text(encoding='utf-8')
    except Exception:
        return set(), set()

    imports = set()
    barrel_symbols = set()

    # Pattern for deep imports (non-barrel)
    # Match: from '@fitglue/shared/dist/...'
    deep_pattern = r"from\s+['\"](@fitglue/shared/[^'\"]+)['\"]"
    for match in re.finditer(deep_pattern, content):
        imports.add(match.group(1))

    # Pattern for barrel imports with named symbols
    # Match: import { Symbol1, Symbol2 } from '@fitglue/shared'
    # Match: import type { Symbol1 } from '@fitglue/shared'
    barrel_pattern = r"import\s+(?:type\s+)?{([^}]+)}\s+from\s+['\"]@fitglue/shared['\"]"
    for match in re.finditer(barrel_pattern, content):
        symbols_str = match.group(1)
        # Parse symbols, handling aliases (e.g., "Foo as Bar")
        for symbol in symbols_str.split(','):
            symbol = symbol.strip()
            if ' as ' in symbol:
                symbol = symbol.split(' as ')[0].strip()
            if symbol:
                barrel_symbols.add(symbol)

    # Pattern for namespace imports
    # Match: import * as shared from '@fitglue/shared'
    namespace_pattern = r"import\s+\*\s+as\s+\w+\s+from\s+['\"]@fitglue/shared['\"]"
    if re.search(namespace_pattern, content):
        # Namespace import = needs everything from barrel
        imports.add('@fitglue/shared')

    return imports, barrel_symbols


def get_handler_imports(handler_dir: Path) -> Tuple[Set[str], Set[str]]:
    """
    Get all @fitglue/shared imports for a handler.

    Scans all .ts files in the handler directory (excluding node_modules, dist, test files).

    Returns:
    - Set of deep import paths
    - Set of symbols imported from barrel
    """
    all_imports = set()
    all_symbols = set()

    for ts_file in handler_dir.rglob("*.ts"):
        # Skip excluded directories
        path_str = str(ts_file)
        if any(excl in path_str for excl in ['node_modules', 'dist', 'build', 'coverage']):
            continue

        imports, symbols = extract_imports(ts_file)
        all_imports.update(imports)
        all_symbols.update(symbols)

    return all_imports, all_symbols


def resolve_modules(
    deep_imports: Set[str],
    barrel_symbols: Set[str],
    modules_config: Dict[str, Any]
) -> Set[str]:
    """
    Resolve import paths and symbols to required module IDs.

    For deep imports: Uses longest-prefix matching to map to specific modules.
    For barrel imports: If any symbols are imported from the barrel, includes
    all barrel-exported modules (since we can't easily map symbols to modules).

    Then recursively adds transitive dependencies.
    """
    required = set()
    import_patterns = modules_config.get("import_patterns", {})
    modules = modules_config.get("modules", {})

    # Handle deep imports (specific paths like @fitglue/shared/dist/types/pb/user)
    for imp in deep_imports:
        # Skip barrel imports - handled separately
        if imp == '@fitglue/shared':
            continue

        # Find matching pattern (longest match wins for specificity)
        best_match = None
        best_len = 0

        for pattern in import_patterns:
            if imp.startswith(pattern) and len(pattern) > best_len:
                best_match = pattern
                best_len = len(pattern)

        if best_match:
            # Add all modules associated with this pattern
            pattern_modules = import_patterns[best_match]
            if isinstance(pattern_modules, list):
                required.update(pattern_modules)
            else:
                required.add(pattern_modules)

    # Handle barrel imports - if any symbols are imported from @fitglue/shared,
    # we need to include the modules that export those symbols.
    # For now, we include all barrel-exported modules (conservative approach).
    # TODO: Implement symbol-to-module mapping for more precise pruning.
    if barrel_symbols or '@fitglue/shared' in deep_imports:
        # Get modules for the barrel pattern
        barrel_modules = import_patterns.get('@fitglue/shared', [])
        if isinstance(barrel_modules, list):
            required.update(barrel_modules)
        else:
            required.add(barrel_modules)

    # Add always-include modules first (so their deps are resolved)
    for mod_id, mod_config in modules.items():
        if mod_config.get("always_include", False):
            required.add(mod_id)

    # Resolve transitive dependencies (iterate until no changes)
    changed = True
    iterations = 0
    max_iterations = 50  # Safety limit

    while changed and iterations < max_iterations:
        changed = False
        iterations += 1

        for mod_id in list(required):
            if mod_id not in modules:
                continue

            deps = modules[mod_id].get("depends_on", [])
            for dep in deps:
                if dep not in required:
                    required.add(dep)
                    changed = True

    return required


def get_module_paths(required_modules: Set[str], modules_config: Dict[str, Any]) -> Set[str]:
    """
    Get all file/directory paths for the required modules.

    Returns relative paths within the shared/ directory.
    """
    modules = modules_config.get("modules", {})
    paths = set()

    for mod_id in required_modules:
        if mod_id not in modules:
            continue

        for path in modules[mod_id].get("paths", []):
            paths.add(path)

    return paths


def analyze_handler(handler_name: str, ts_src_dir: Path) -> Dict[str, Any]:
    """
    Analyze a single handler and return its module requirements.

    Returns dict with:
    - deep_imports: set of deep import paths
    - barrel_symbols: set of symbols imported from barrel
    - modules: set of required module IDs
    - paths: set of paths to include from shared/
    """
    handler_dir = ts_src_dir / handler_name
    modules_config = load_modules_config()

    deep_imports, barrel_symbols = get_handler_imports(handler_dir)
    required_modules = resolve_modules(deep_imports, barrel_symbols, modules_config)
    required_paths = get_module_paths(required_modules, modules_config)

    return {
        'deep_imports': deep_imports,
        'barrel_symbols': barrel_symbols,
        'modules': required_modules,
        'paths': required_paths
    }


def main():
    """CLI entry point for testing import analysis."""
    import sys

    script_dir = Path(__file__).parent
    ts_src_dir = script_dir.parent / "src" / "typescript"

    # If handler name provided, analyze just that handler
    if len(sys.argv) > 1:
        handler_name = sys.argv[1]
        result = analyze_handler(handler_name, ts_src_dir)

        print(f"\n=== Analysis for {handler_name} ===\n")

        if result['deep_imports']:
            print("Deep imports found:")
            for imp in sorted(result['deep_imports']):
                print(f"  - {imp}")
        else:
            print("No deep imports found.")

        if result['barrel_symbols']:
            print(f"\nBarrel imports ({len(result['barrel_symbols'])} symbols):")
            for sym in sorted(result['barrel_symbols']):
                print(f"  - {sym}")
        else:
            print("\nNo barrel imports.")

        print(f"\nRequired modules ({len(result['modules'])}):")
        for mod in sorted(result['modules']):
            print(f"  - {mod}")

        print(f"\nPaths to include ({len(result['paths'])}):")
        for path in sorted(result['paths']):
            print(f"  - {path}")

        return

    # Otherwise, analyze all handlers
    modules_config = load_modules_config()

    # Auto-discover handlers (same logic as build script)
    excluded_dirs = {'shared', 'admin-cli', 'mcp-server', 'node_modules', 'parkrun-fetcher'}
    handlers = []

    for item in sorted(ts_src_dir.iterdir()):
        if item.is_dir() and item.name not in excluded_dirs:
            if (item / 'package.json').exists():
                handlers.append(item.name)

    print(f"Analyzing {len(handlers)} handlers...\n")

    # Track handlers that use only deep imports (can be pruned more aggressively)
    handlers_with_barrel = []
    handlers_deep_only = []

    # Build summary
    all_modules = modules_config.get("modules", {})
    module_usage = {mod_id: [] for mod_id in all_modules}

    for handler_name in handlers:
        result = analyze_handler(handler_name, ts_src_dir)
        modules = result['modules']

        uses_barrel = bool(result['barrel_symbols'])
        status = "📦 barrel" if uses_barrel else "🎯 deep only"
        print(f"{handler_name}: {len(modules)} modules {status}")

        if uses_barrel:
            handlers_with_barrel.append(handler_name)
        else:
            handlers_deep_only.append(handler_name)

        for mod_id in modules:
            if mod_id in module_usage:
                module_usage[mod_id].append(handler_name)

    print("\n=== Summary ===\n")
    print(f"Handlers using barrel imports (need migration): {len(handlers_with_barrel)}")
    print(f"Handlers using only deep imports (optimal): {len(handlers_deep_only)}")

    print("\n=== Module Usage Summary ===\n")
    for mod_id, users in sorted(module_usage.items(), key=lambda x: len(x[1]), reverse=True):
        if users:
            print(f"{mod_id}: {len(users)} handlers")
        else:
            print(f"{mod_id}: unused")


if __name__ == "__main__":
    main()
