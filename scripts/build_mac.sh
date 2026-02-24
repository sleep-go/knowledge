#!/bin/bash

APP_NAME="KnowledgeBase"
APP_DIR="$APP_NAME.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"

echo "Building $APP_NAME for macOS..."

# Create directory structure
mkdir -p "$MACOS_DIR"
mkdir -p "$RESOURCES_DIR"

# Check for CGO/GCC
if ! command -v gcc &> /dev/null; then
    echo "Warning: gcc not found. Building in Mock mode (no llama.cpp)."
    go build -o "$MACOS_DIR/knowledge" ./cmd/server
else
    echo "GCC found. Building with embedded llama.cpp..."
    CGO_ENABLED=1 go build -tags cgo_llama -o "$MACOS_DIR/knowledge" ./cmd/server
fi

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Create Info.plist
cat > "$CONTENTS_DIR/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>knowledge</string>
    <key>CFBundleIdentifier</key>
    <string>com.example.knowledgebase</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSUIElement</key>
    <true/>
</dict>
</plist>
EOF

echo "Build successful! Application bundle: $APP_DIR"
