set -e
set -u
set -o pipefail

DEST_FILE=src/teslacoil.ts
node node_modules/restful-react/dist/bin/restful-react.js import --file teslacoil.openapi.json \
    --output $DEST_FILE --skip-react

echo -e "/* eslint-disable */

$(cat $DEST_FILE)" > $DEST_FILE