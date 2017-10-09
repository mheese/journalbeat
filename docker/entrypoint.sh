#!/bin/bash -e
if [[ -z $1 ]] || [[ ${1:0:1} == '-' ]] ; then
  exec  /journalbeat -e "$@"
else
  exec "$@"
fi
