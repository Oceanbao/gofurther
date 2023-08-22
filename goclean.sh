#!/bin/bash
set -euo pipefail

function traverse() {
  for file in "$1"/*
  do
    if [[ ! -d "${file}" ]]; then
      isELF ${file} &&\
        echo "${file} is ELF" &&\
        rm -rf "${file}" \
        || true
    else
      echo "entering recursion with: ${file}"
      traverse "${file}"
    fi
  done
}

function main() {
  traverse "$1"
}

function isELF() {
  file $1 | grep -e '.*ELF.*executable.*'
}

main "$1"
