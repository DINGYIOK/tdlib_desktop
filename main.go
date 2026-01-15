package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "财财客户端",
		Width:  1100,
		Height: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		// Debug: options.Debug{
		// 	OpenInspectorOnStartup: true,
		// },
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

/**
1. 无法删除 登陆超时
2. 发送验证码按钮/ 登陆按钮

*/

/*
export PATH=$PATH:~/go/bin

export CGO_ENABLED=1
export OPENSSL_PREFIX=$(brew --prefix openssl@3)
export CGO_CFLAGS="-I$(pwd)/tdlib-macos-arm64-bin/include"
export CGO_LDFLAGS="-L$(pwd)/tdlib-macos-arm64-bin/lib -ltdjson -L${OPENSSL_PREFIX}/lib -lssl -lcrypto"
export DYLD_LIBRARY_PATH=$DYLD_LIBRARY_PATH:$(pwd)/tdlib-macos-arm64-bin/lib
export CGO_LDFLAGS="-L$(pwd)/tdlib-macos-arm64-bin/lib -ltdjson -Wl,-rpath,$(pwd)/tdlib-macos-arm64-bin/lib -L${OPENSSL_PREFIX}/lib -lssl -lcrypto"

*/
