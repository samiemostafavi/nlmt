#!/bin/sh

# This script may be used during development for making builds and generating doc.
# Requirements:
# - stringer (go get -u -a golang.org/x/tools/cmd/stringer)
# - pandoc (apt-get install pandoc OR brew install pandoc)

action="build"
pkg="github.com/samiemostafavi/nlmt/cmd/nlmt"
ldflags=""
linkshared=""
tags=""
race=""
env=""

# html filter
html_filter() {
	sed 's/<table>/<table class="pure-table pure-table-striped">/g'
}

# interpret keywords
for a in $*; do
	case "$a" in
		"install") action="install"
		ldflags="$ldflags -s -w"
		;;
		"nobuild") nobuild="1"
		;;
		"nodoc") nodoc="1"
		;;
		"min") ldflags="$ldflags -s -w"
		;;
		"linkshared") linkshared="-linkshared"
		;;
		"race") race="-race"
		;;
		"profile") tags="$tags profile"
		;;
		"prod") tags="$tags prod"
		;;
		"linux-386"|"linux32") env="GOOS=linux GOARCH=386"
		;;
		"linux-387"|"linux-alix") env="GOOS=linux GOARCH=386 GO386=387"
		;;
		"linux-amd64"|"linux") env="GOOS=linux GOARCH=amd64"
		;;
		"linux-arm"|"rpi") env="GOOS=linux GOARCH=arm"
		;;
		"linux-mips64"|"erl") env="GOOS=linux GOARCH=mips"
		;;
		"linux-mipsle"|"erx") env="GOOS=linux GOARCH=mipsle"
		;;
		"linux-mips-softfloat"|"om2p") env="GOOS=linux GOARCH=mips GOMIPS=softfloat"
		;;
		"darwin-amd64"|"osx") env="GOOS=darwin GOARCH=amd64"
		;;
		"win32"|"windows32") env="GOOS=windows GOARCH=386"
		;;
		"win"|"windows") env="GOOS=windows GOARCH=amd64"
		;;
		*) echo "Unknown parameter: $a"
		exit 1
		;;
	esac
done

# build source
if [ -z "$nobuild" ]; then
	go generate
	eval $env go $action -tags \'$tags\' $race -ldflags=\'$ldflags\' $linkshared $pkg
fi

# generate docs
if [ -z "$nodoc" ]; then
	for f in irtt irtt-client irtt-server; do
		pandoc -s -t man doc/$f.md -o doc/$f.1
		pandoc -t html -H doc/head.html doc/$f.md | html_filter > doc/$f.html
	done
fi
