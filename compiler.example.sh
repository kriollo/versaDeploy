#!/bin/bash
# Example custom frontend compiler for Vue.js files
# This demonstrates how to create a custom compiler that versaDeploy will invoke

FILE="$1"

if [ -z "$FILE" ]; then
    echo "Usage: compiler.sh <file>"
    exit 1
fi

# Extract filename without extension
BASENAME=$(basename "$FILE" .vue)
DIRNAME=$(dirname "$FILE")

# Output file in public directory
OUTPUT="public/$BASENAME.js"

echo "Compiling $FILE → $OUTPUT"

# Example: Use a custom Vue compiler (replace with your actual compiler)
# This is a placeholder - implement your actual compilation logic
cat "$FILE" | sed 's/\.vue/.js/g' > "$OUTPUT"

# Rewrite imports to point to node_modules
# Example: from "vue" → from "./node_modules/vue/dist/vue.esm.js"
sed -i 's|from "vue"|from "./node_modules/vue/dist/vue.esm.js"|g' "$OUTPUT"
sed -i 's|from "vuex"|from "./node_modules/vuex/dist/vuex.esm.js"|g' "$OUTPUT"

echo "✓ Compiled $FILE successfully"
