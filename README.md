<h1>JS/TS client library for Teslacoil API</h1>

This is a client library for interacting with the Teslacoil APIs for sending and receiving bitcoin payments. See our [API docs](https://teslacoil.io/api) for more information. We are still in beta, and are not open for users yet.

- [Add it to your project:](#add-it-to-your-project)
  - [Yarn](#yarn)
  - [npm](#npm)
- [Usage:](#usage)
  - [Setting up API keys](#setting-up-api-keys)
  - [Request data from REST APIs](#request-data-from-rest-apis)
- [Publishing](#publishing)

## Add it to your project:

#### Yarn

```bash
$ yarn add teslacoil
```

#### npm

```bash
$ npm install --save teslacoil
```

## Usage:

### Setting up api keys

To get an API key, you first need to register an account on [teslacoil.io/signup](teslacoil.io/signup). After registering, use the GUI to create an API-key. The website will guide you through the process.
When you have your API-key, you can get started.

```typescript
import * as teslacoil from "teslacoil";

teslacoil.setCredentials(`YOUR-API-KEY`);
```

The default network is mainnet, but testnet is also supported. You can use the API on testnet by doing:

```typescript
import * as teslacoil from "teslacoil";

teslacoil.setCredentials(`YOUR-API-KEY`, "TESTNET");
```

### Request data from REST APIs

```typescript
import * as teslacoil from "teslacoil";

teslacoil.setCredentials(`YOUR-API-KEY`, "TESTNET");

// decode an invoice
const decodedInvoice = await teslacoil.decodeInvoice("insert payment request here");

// create a invoice for 5000 sats
const invoice = await teslacoil.createInvoice({ amountSat: 5000 });
```

As of yet, only four features are supported, and we are updating this on an ongoing basis.
To see all of the features, see our [API docs](https://teslacoil.io/api). Here you will find complete code samples for making requests, as well as what responses look like, for all API endpoints and request types. All request and response types are also present in this library and can be found in [src/teslacoil.ts](https://github.com/ArcaneCryptoAS/teslacoil/blob/master/src/teslacoil.ts)

## Publishing

```
$ yarn publish
```

Lints, checks formatting, makes new git tag, pushes new git tag, build, pushes build to npm
