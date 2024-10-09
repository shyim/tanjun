gen-completions:
	rm -rf completions
	mkdir completions
	go run . completion bash > completions/tanjun.bash
	go run . completion zsh > completions/tanjun.zsh
	go run . completion fish > completions/tanjun.fish

gen-schema:
	go run internal/schema-generator/main.go
