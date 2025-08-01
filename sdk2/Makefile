# MPC SDK2 Makefile for Android and iOS
# 用于构建Android和iOS的so文件

# Go编译器设置
GO = go
CGO_ENABLED = 1

# 输出目录
BUILD_DIR = build
ANDROID_DIR = $(BUILD_DIR)/android
IOS_DIR = $(BUILD_DIR)/ios

# 库名称
LIB_NAME = libmpc

# Android架构
ANDROID_ARCHS = arm64-v8a armeabi-v7a x86 x86_64

# iOS架构
IOS_ARCHS = arm64 x86_64

# 默认目标
.PHONY: all clean android ios test

all: android ios

# 创建构建目录
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(ANDROID_DIR): $(BUILD_DIR)
	mkdir -p $(ANDROID_DIR)

$(IOS_DIR): $(BUILD_DIR)
	mkdir -p $(IOS_DIR)

# ================================
# Android构建目标
# ================================

android: $(ANDROID_DIR)
	@echo "Building Android libraries..."
	@$(MAKE) android-arm64-v8a
	@$(MAKE) android-armeabi-v7a
	@$(MAKE) android-x86
	@$(MAKE) android-x86_64
	@echo "Android libraries built successfully!"

# Android ARM64
android-arm64-v8a: $(ANDROID_DIR)
	@echo "Building Android ARM64-v8a..."
	@mkdir -p $(ANDROID_DIR)/arm64-v8a
	CGO_ENABLED=1 \
	GOOS=android \
	GOARCH=arm64 \
	CC=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android21-clang \
	CXX=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android21-clang++ \
	$(GO) build -buildmode=c-shared -o $(ANDROID_DIR)/arm64-v8a/$(LIB_NAME).so ./mpc_bridge.go

# Android ARMv7
android-armeabi-v7a: $(ANDROID_DIR)
	@echo "Building Android ARMv7..."
	@mkdir -p $(ANDROID_DIR)/armeabi-v7a
	CGO_ENABLED=1 \
	GOOS=android \
	GOARCH=arm \
	GOARM=7 \
	CC=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi21-clang \
	CXX=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi21-clang++ \
	$(GO) build -buildmode=c-shared -o $(ANDROID_DIR)/armeabi-v7a/$(LIB_NAME).so ./mpc_bridge.go

# Android x86
android-x86: $(ANDROID_DIR)
	@echo "Building Android x86..."
	@mkdir -p $(ANDROID_DIR)/x86
	CGO_ENABLED=1 \
	GOOS=android \
	GOARCH=386 \
	CC=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/i686-linux-android21-clang \
	CXX=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/i686-linux-android21-clang++ \
	$(GO) build -buildmode=c-shared -o $(ANDROID_DIR)/x86/$(LIB_NAME).so ./mpc_bridge.go

# Android x86_64
android-x86_64: $(ANDROID_DIR)
	@echo "Building Android x86_64..."
	@mkdir -p $(ANDROID_DIR)/x86_64
	CGO_ENABLED=1 \
	GOOS=android \
	GOARCH=amd64 \
	CC=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/x86_64-linux-android21-clang \
	CXX=$(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/darwin-x86_64/bin/x86_64-linux-android21-clang++ \
	$(GO) build -buildmode=c-shared -o $(ANDROID_DIR)/x86_64/$(LIB_NAME).so ./mpc_bridge.go

# ================================
# iOS构建目标
# ================================

ios: $(IOS_DIR)
	@echo "Building iOS libraries..."
	@$(MAKE) ios-arm64
	@$(MAKE) ios-x86_64
	@$(MAKE) ios-universal
	@echo "iOS libraries built successfully!"

# iOS ARM64 (设备)
ios-arm64: $(IOS_DIR)
	@echo "Building iOS ARM64..."
	@mkdir -p $(IOS_DIR)/arm64
	CGO_ENABLED=1 \
	GOOS=ios \
	GOARCH=arm64 \
	CGO_CFLAGS="-arch arm64 -mios-version-min=11.0 -isysroot $(shell xcrun --sdk iphoneos --show-sdk-path)" \
	CGO_LDFLAGS="-arch arm64 -mios-version-min=11.0 -isysroot $(shell xcrun --sdk iphoneos --show-sdk-path)" \
	CC=$(shell xcrun --sdk iphoneos --find clang) \
	CXX=$(shell xcrun --sdk iphoneos --find clang++) \
	$(GO) build -buildmode=c-archive -o $(IOS_DIR)/arm64/$(LIB_NAME).a ./mpc_bridge.go

# iOS x86_64 (模拟器)
ios-x86_64: $(IOS_DIR)
	@echo "Building iOS x86_64..."
	@mkdir -p $(IOS_DIR)/x86_64
	CGO_ENABLED=1 \
	GOOS=ios \
	GOARCH=amd64 \
	CGO_CFLAGS="-arch x86_64 -mios-simulator-version-min=11.0 -isysroot $(shell xcrun --sdk iphonesimulator --show-sdk-path)" \
	CGO_LDFLAGS="-arch x86_64 -mios-simulator-version-min=11.0 -isysroot $(shell xcrun --sdk iphonesimulator --show-sdk-path)" \
	CC=$(shell xcrun --sdk iphonesimulator --find clang) \
	CXX=$(shell xcrun --sdk iphonesimulator --find clang++) \
	$(GO) build -buildmode=c-archive -o $(IOS_DIR)/x86_64/$(LIB_NAME).a ./mpc_bridge.go

# iOS通用库 (合并ARM64和x86_64)
ios-universal: ios-arm64 ios-x86_64
	@echo "Creating iOS universal library..."
	@mkdir -p $(IOS_DIR)/universal
	lipo -create \
		$(IOS_DIR)/arm64/$(LIB_NAME).a \
		$(IOS_DIR)/x86_64/$(LIB_NAME).a \
		-output $(IOS_DIR)/universal/$(LIB_NAME).a
	@cp $(IOS_DIR)/arm64/$(LIB_NAME).h $(IOS_DIR)/universal/$(LIB_NAME).h

# ================================
# 测试目标
# ================================

test: test-c test-go test-examples

# 测试C接口
test-c: $(BUILD_DIR)
	@echo "Building C test..."
	gcc -I. -L$(BUILD_DIR) -o $(BUILD_DIR)/test_c test_mpc_c.c mpc_c_interface.c -lmpc
	@echo "Running C test..."
	@$(BUILD_DIR)/test_c

# 测试Go接口
test-go:
	@echo "Running Go tests..."
	$(GO) test -v ./...

# 构建示例程序
test-examples: build-local
	@echo "Building example programs..."
	@mkdir -p $(BUILD_DIR)/examples
	gcc -I. -L. -o $(BUILD_DIR)/examples/ed25519_sign_example ed25519_sign_example.c -lmpc -lpthread
	gcc -I. -L. -o $(BUILD_DIR)/examples/complete_mpc_example complete_mpc_example.c -lmpc -lpthread
	@echo "Example programs built successfully!"

# 运行示例程序
run-examples: test-examples
	@echo "Running Ed25519 signing example..."
	@LD_LIBRARY_PATH=. $(BUILD_DIR)/examples/ed25519_sign_example
	@echo ""
	@echo "Running complete MPC workflow example..."
	@LD_LIBRARY_PATH=. $(BUILD_DIR)/examples/complete_mpc_example

# 构建本地测试库
build-local: $(BUILD_DIR)
	@echo "Building local shared library for testing..."
	CGO_ENABLED=1 $(GO) build -buildmode=c-shared -o $(LIB_NAME).so ./mpc_bridge.go
	@echo "Local library built: $(LIB_NAME).so"

# ================================
# 清理目标
# ================================

clean:
	@echo "Cleaning build directory..."
	rm -rf $(BUILD_DIR)

# ================================
# 帮助信息
# ================================

help:
	@echo "MPC SDK2 Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  all              - Build all libraries (Android + iOS)"
	@echo "  android          - Build all Android libraries"
	@echo "  android-arm64-v8a - Build Android ARM64 library"
	@echo "  android-armeabi-v7a - Build Android ARMv7 library"
	@echo "  android-x86      - Build Android x86 library"
	@echo "  android-x86_64   - Build Android x86_64 library"
	@echo "  ios              - Build all iOS libraries"
	@echo "  ios-arm64        - Build iOS ARM64 library"
	@echo "  ios-x86_64       - Build iOS x86_64 library"
	@echo "  ios-universal    - Build iOS universal library"
	@echo "  test             - Run all tests"
	@echo "  test-c           - Run C interface tests"
	@echo "  test-go          - Run Go tests"
	@echo "  test-examples    - Build example programs"
	@echo "  run-examples     - Build and run example programs"
	@echo "  build-local      - Build local shared library for testing"
	@echo "  clean            - Clean build directory"
	@echo "  help             - Show this help message"
	@echo ""
	@echo "Prerequisites:"
	@echo "  - Go 1.19+ installed"
	@echo "  - Android NDK (set ANDROID_NDK_HOME environment variable)"
	@echo "  - Xcode command line tools (for iOS builds)"
	@echo "  - GCC (for building examples and tests)"
	@echo ""
	@echo "Environment Variables:"
	@echo "  ANDROID_NDK_HOME - Path to Android NDK"
	@echo ""
	@echo "Example usage:"
	@echo "  make android      # Build all Android libraries"
	@echo "  make ios          # Build all iOS libraries"
	@echo "  make build-local  # Build local library for testing"
	@echo "  make run-examples # Build and run example programs"
	@echo "  make clean        # Clean build directory"

# 检查环境变量
check-android-env:
ifndef ANDROID_NDK_HOME
	$(error ANDROID_NDK_HOME is not set. Please set it to your Android NDK path)
endif

# 为Android目标添加环境检查依赖
android-arm64-v8a: check-android-env
android-armeabi-v7a: check-android-env
android-x86: check-android-env
android-x86_64: check-android-env