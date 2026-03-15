package main

import "github.com/ntthienan0507-web/go-api-template/cmd"

// @title           go-api-template API
// @version         1.0
// @description     API server for go-api-template
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	cmd.Run()
}
