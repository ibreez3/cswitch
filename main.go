package main

import (
	// 导入所有模块以触发 provider 注册
	_ "github.com/ibreez3/cswitch/provider"

	"github.com/ibreez3/cswitch/cmd"
)

func main() {
	cmd.Execute()
}
