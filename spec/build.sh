#!/bin/sh
find . -name '*.md' |
    sort -n |
    xargs pandoc -N --toc --self-contained -o "spec.${1:-pdf}"
