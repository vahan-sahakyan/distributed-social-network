module github.com/vahan-sahakyan/distributed-social-network/feed-service

go 1.24

require (
	github.com/bradfitz/gomemcache v0.0.0-20230905024940-24af94b03874
	github.com/gofiber/fiber/v2 v2.52.5
	github.com/segmentio/kafka-go v0.4.47
	github.com/vahan-sahakyan/distributed-social-network/pkg v0.0.0
)

require (
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/stretchr/testify v1.8.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.51.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/vahan-sahakyan/distributed-social-network/pkg => ../../pkg
