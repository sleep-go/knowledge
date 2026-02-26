#!/bin/bash
set -e

# Configuration
APP_NAME="Knowledge"
OUTPUT_DIR="bin"
APP_DIR="$OUTPUT_DIR/$APP_NAME.app"
BINARY_NAME="knowledge"
SRC_DIR="cmd/server/main.go"

# 1. Compile the Go binary
echo "Compiling Go binary..."
# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Ensure CGO is enabled as required by binding.go
export CGO_ENABLED=1

if ! go build -o "$OUTPUT_DIR/$BINARY_NAME" "$SRC_DIR"; then
    echo "Error: Compilation failed."
    exit 1
fi

# 2. Create the Knowledge.app directory structure
echo "Creating app bundle structure..."
if [ -d "$APP_DIR" ]; then
    rm -rf "$APP_DIR"
fi
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# 3. Generate a minimal Info.plist
echo "Generating Info.plist..."
cat > "$APP_DIR/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$BINARY_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>com.example.knowledge</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

# 4. Copy the binary to Knowledge.app/Contents/MacOS/knowledge
echo "Copying binary..."
cp "$OUTPUT_DIR/$BINARY_NAME" "$APP_DIR/Contents/MacOS/$BINARY_NAME"

# 5. Copy resources (models and data)
echo "Copying resources..."
mkdir -p "$APP_DIR/Contents/Resources/models"
mkdir -p "$APP_DIR/Contents/Resources/data"

if [ -d "models" ]; then
    cp -r models/* "$APP_DIR/Contents/Resources/models/"
    echo "Copied models"
else
    echo "Warning: models directory not found"
fi

if [ -d "data" ]; then
    cp -r data/* "$APP_DIR/Contents/Resources/data/"
    echo "Copied data"
else
    # Create empty db if data dir doesn't exist
    touch "$APP_DIR/Contents/Resources/data/.keep"
fi

# 6. Make the binary executable
echo "Setting permissions..."
chmod +x "$APP_DIR/Contents/MacOS/$BINARY_NAME"

# 6. Print success message
echo "Success! $APP_NAME.app has been created at $APP_DIR"
