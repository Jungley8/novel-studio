//go:build !production

package main

// defaultIsDevBuild 为开发模式构建标识（非 production tag 构建默认开启开发模式）
const defaultIsDevBuild = true
const defaultDevPort = 28981
