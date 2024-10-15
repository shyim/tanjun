package buildpack

func generateByGolang() (*GeneratedImageResult, error) {
	var result GeneratedImageResult

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest as builder")
	result.AddLine("RUN apk add --no-cache go")
	result.NewLine()
	result.AddLine("WORKDIR /app")
	result.AddLine("COPY . .")
	result.AddLine("RUN go build -o /app/app")
	result.NewLine()

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest")
	result.AddLine("WORKDIR /app")
	result.AddLine("COPY --from=builder /app/app /app")

	result.AddLine("ENV PORT=3000")
	result.AddLine("CMD /app")
	result.AddLine("EXPOSE 3000")

	return &result, nil
}
