FROM golang:1.5.1

ENV GOOS linux
ENV GOPATH /usr/src/app

WORKDIR /usr/src/app

COPY . src/github.com/resin-io/resin-supervisor/gosuper

RUN chmod +x src/github.com/resin-io/resin-supervisor/gosuper/build_gosuper.sh \
	&& chmod +x src/github.com/resin-io/resin-supervisor/gosuper/test_gosuper.sh \
	&& chmod +x src/github.com/resin-io/resin-supervisor/gosuper/test_formatting.sh

# Run go install with -a (force rebuilding of all packages)
# and -v (print package names as they are built)
CMD cd ./src/github.com/resin-io/resin-supervisor/gosuper && ./build_gosuper.sh
