#!/bin/bash
set -e

# Find all Go files in src/go
GO_FILES=$(find src/go -name "*.go" -not -path "*/vendor/*" -not -path "*/api/*/*.gen.go")

# Pattern to search for: json.Unmarshal or json.Marshal
# We want to flag usage where the variable being marshalled/unmarshalled is likely a Protobuf type.
# This is hard to do perfectly with grep, but we can flag ANY json.Marshal/Unmarshal in files that import "github.com/fitglue/server/src/go/pkg/types/pb".

echo "Checking for potential Protobuf JSON misuse..."
FAILED=0

for file in $GO_FILES; do
    # Check if file imports the pb package
    if grep -q "github.com/fitglue/server/src/go/pkg/types/pb" "$file"; then
        # Check if file uses encoding/json
        if grep -q "\"encoding/json\"" "$file"; then
            echo "WARNING: $file imports protobuf types AND encoding/json. Verify usage."
            # We can be more aggressive: fail if json.Marshal/Unmarshal is used directly?
            # Let's just list the lines for now.
            grep -nE "json\.(Marshal|Unmarshal)" "$file" || true
             # Determine if this should be a failure. ideally yes, but there might be valid mixed usage.
             # For now, let's just warn.
        fi
    fi
done

exit $FAILED
