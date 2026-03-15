package main

import "github.com/ntthienan0507-web/gostack-kit/cmd"

// @title           gostack-kit API
// @version         1.0
// @description     API server for gostack-kit
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	cmd.Run()
}
