package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// assets 嵌入前端构建产物，保证应用可被打包为单文件可执行程序。
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application := NewApp()

	err := wails.Run(&options.App{
		Title:             "TreeShift",
		Width:             760,
		Height:            552,
		MinWidth:          560,
		MinHeight:         456,
		DisableResize:     false,
		Frameless:         false,
		BackgroundColour:  &options.RGBA{R: 245, G: 241, B: 233, A: 1},
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         application.startup,
		Bind:              []interface{}{application},
	})
	if err != nil {
		log.Fatalf("启动 TreeShift 失败: %v", err)
	}
}
