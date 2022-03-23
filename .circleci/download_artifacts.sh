#!/bin/bash

export CIRCLE_TOKEN='d8bad1b9cbfe1db266035f37fb84e13cc14070de'

curl https://circleci.com/api/v1.1/project/github/arcology-network/arbitrator-engine/latest/artifacts?circle-token=$CIRCLE_TOKEN \
   | grep -o 'https://[^"]*' \
   | sed -e "s/$/?circle-token=$CIRCLE_TOKEN/" \
   | wget -O libarbitrator.so -v -i -