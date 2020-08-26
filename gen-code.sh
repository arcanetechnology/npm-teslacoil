set -e
set -u
set -o pipefail

DEST_FILE=src/teslacoil.ts
node node_modules/restful-react/dist/bin/restful-react.js import --file teslacoil.swagger.json \
    --output $DEST_FILE --ts

echo -e "/* eslint-disable */

import axios from \"axios\";

const api = axios.create({
  validateStatus: () => true,
});

const apiKeyNotSetMessage = \"looks like you haven't set your api-key! set api-key by calling setCredentials(key)\";
type environments = \"MAINNET\" | \"TESTNET\";
let apiKey = \"\";

export const setCredentials = (key: string, network: environments = \"MAINNET\"): void => {
  if (key === \"\") {
    throw Error(\"api key can not be set to empty string\");
  }

  apiKey = key;
  api.defaults.baseURL = network === \"MAINNET\" ? \"https://api.teslacoil.io\" : \"https://testnetapi.teslacoil.io\";
  api.defaults.timeout = 5000;
  api.defaults.headers = { Authorization: apiKey };
};

$(cat $DEST_FILE)" > $DEST_FILE