GOCACHE     ?= /tmp/go-build-cache
ANDROID_HOME ?= $(HOME)/Android/Sdk
GOTOOLCHAIN  ?= go1.25.0
GOFLAGS      ?= -buildvcs=false
AAR_OUT       = android/app/libs/mobile.aar
APK_VERSION   = 1.5.1-pre8
APK_RELEASE   = android/app/build/outputs/apk/release/zyrln-$(APK_VERSION).apk
DESKTOP_VERSION ?= $(APK_VERSION)
DIST_DIR      = dist
DESKTOP_LINUX = $(DIST_DIR)/zyrln-$(DESKTOP_VERSION)-linux-amd64
DESKTOP_WIN   = $(DIST_DIR)/zyrln-$(DESKTOP_VERSION)-windows-amd64.exe
DESKTOP_MAC_ARM64 = $(DIST_DIR)/zyrln-$(DESKTOP_VERSION)-darwin-arm64
DESKTOP_MAC_AMD64 = $(DIST_DIR)/zyrln-$(DESKTOP_VERSION)-darwin-amd64

export ANDROID_HOME
export GOTOOLCHAIN
export GOFLAGS

.PHONY: all desktop desktop-release desktop-linux desktop-windows desktop-macos desktop-macos-arm64 desktop-macos-amd64 gui proxy test android keystore clean

all: desktop

## Build the desktop CLI binary.
desktop:
	GOCACHE=$(GOCACHE) go build -o zyrln ./platforms/desktop/

## Build all desktop release binaries into dist/.
desktop-release: desktop-linux desktop-windows desktop-macos
	@echo "Desktop release binaries:"
	@ls -lh $(DESKTOP_LINUX) $(DESKTOP_WIN) $(DESKTOP_MAC_ARM64) $(DESKTOP_MAC_AMD64)

## Build the Linux desktop binary.
desktop-linux:
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 GOCACHE=$(GOCACHE) go build -o $(DESKTOP_LINUX) ./platforms/desktop/
	@echo "Linux → $(DESKTOP_LINUX)"

## Build the Windows desktop binary.
desktop-windows:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 GOCACHE=$(GOCACHE) go build -o $(DESKTOP_WIN) ./platforms/desktop/
	@echo "Windows → $(DESKTOP_WIN)"

## Build both macOS desktop binaries.
desktop-macos: desktop-macos-arm64 desktop-macos-amd64

## Build the macOS Apple Silicon desktop binary.
desktop-macos-arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=arm64 GOCACHE=$(GOCACHE) go build -o $(DESKTOP_MAC_ARM64) ./platforms/desktop/
	@echo "macOS arm64 → $(DESKTOP_MAC_ARM64)"

## Build the macOS Intel desktop binary.
desktop-macos-amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 GOCACHE=$(GOCACHE) go build -o $(DESKTOP_MAC_AMD64) ./platforms/desktop/
	@echo "macOS amd64 → $(DESKTOP_MAC_AMD64)"

## Start the desktop relay proxy (reads config.env).
proxy:
	@if [ ! -f certs/zyrln-ca.pem ]; then \
		echo "CA certificate not found. Run this first:"; \
		echo "  make desktop && ./zyrln -init-ca"; \
		exit 1; \
	fi
	GOCACHE=$(GOCACHE) go run ./platforms/desktop/ -serve-proxy

## Start the browser-based GUI.
gui: desktop
	./zyrln -gui

## Smoke test the full relay chain.
test:
	@if [ ! -f config.env ]; then \
		echo "config.env not found. Create it with:"; \
		echo "  fronted-appscript-url = https://script.google.com/macros/s/YOUR_ID/exec"; \
		echo "  auth-key              = YOUR_KEY"; \
		exit 1; \
	fi
	GOCACHE=$(GOCACHE) go run ./platforms/desktop/ -relay-fetch-url 'https://www.gstatic.com/generate_204'

## Generate a release signing keystore (run once before `make android`).
## Requires: keytool (comes with the JDK)
keystore:
	@if [ -f android/keystore.properties ]; then \
		echo "Keystore already exists. Delete android/zyrln.jks and android/keystore.properties to regenerate."; \
		exit 1; \
	fi
	keytool -genkeypair -v \
		-keystore android/zyrln.jks \
		-alias zyrln \
		-keyalg RSA -keysize 2048 -validity 10000 \
		-storepass zyrln123 -keypass zyrln123 \
		-dname "CN=Zyrln, O=Zyrln, C=US"
	@printf 'storeFile=../zyrln.jks\nstorePassword=zyrln123\nkeyAlias=zyrln\nkeyPassword=zyrln123\n' \
		> android/keystore.properties
	@echo "Keystore → android/zyrln.jks"
	@echo "Properties → android/keystore.properties"

## Build the release APK (requires keystore first).
android:
	@if [ ! -f android/keystore.properties ]; then \
		echo "Keystore not found. Run this first:"; \
		echo "  make keystore"; \
		exit 1; \
	fi
	@if ! command -v gomobile >/dev/null 2>&1 && [ ! -f $(HOME)/go/bin/gomobile ]; then \
		echo "gomobile not found. Run this first:"; \
		echo "  go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init"; \
		exit 1; \
	fi
	@echo "Building gomobile AAR..."
	@mkdir -p android/app/libs
	PATH=$(PATH):$(HOME)/go/bin GOCACHE=$(GOCACHE) gomobile bind \
		-target android \
		-androidapi 21 \
		-o $(AAR_OUT) \
		zyrln/platforms/mobile
	cd android && ./gradlew assembleRelease
	@echo "APK → $(APK_RELEASE)"

clean:
	rm -f zyrln $(AAR_OUT) $(DESKTOP_LINUX) $(DESKTOP_WIN) $(DESKTOP_MAC_ARM64) $(DESKTOP_MAC_AMD64)
	cd android && ./gradlew clean 2>/dev/null || true
