
GOPATH := ${GOPATH}:${PWD}

all: compile

compile:
	@go build -v bread

deps:
	go get -v github.com/mattn/go-sqlite3

fmt:
	go fmt bread bread/db bread/nbc bread/rss bread/session bread/story bread/index bread/config cache

docs:
	godoc -http=:6060 &

test: 
	@go test -i bread/nbc
	go test bread/nbc
	@go test -i bread/session
	go test bread/session
	@go test -i cache
	go test cache

dist: compile
	tar cjf bread.tar.bz2 bread db/bread.sql static templates

