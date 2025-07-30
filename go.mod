module github.com/alpsilva/go-blog-aggregator.git

go 1.24.5

replace github.com/alpsilva/config v0.0.0 => ./internal/config

require github.com/alpsilva/config v0.0.0

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
)
