.PHONY: default
default:
	aws-vault exec trab.canv.dev -- go run main.go
