#!/bin/bash

MAKE="/usr/bin/make"

PROTOTYPE="$(pwd)/prototype"

echo Using prototype directory: ${PROTOTYPE}
${MAKE} BR2_EXTERNAL=${PROTOTYPE} ${@}
