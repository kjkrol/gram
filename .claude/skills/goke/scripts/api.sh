#!/bin/sh
# Prints goke's API from the local checkout joined by go.work: the package's names, or one
# type with its methods.  Usage: api.sh [Type]   (run from anywhere)
cd /home/karol/CODE/goke || exit 1
if [ -n "$1" ]; then
	go doc "$1"
else
	go doc -short .
fi
