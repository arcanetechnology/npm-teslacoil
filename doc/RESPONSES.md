## Standardized API responses

All API responses sent from Teslacoil follow the same format. It looks like this:

```json
{
  "result": {}, // if successfull

  // if any errors occured
  "error": {
    "message": "string",
    "code": "string",
    "fields": [
      {
        "field": "string",
        "message": "string",
        "code": "string"
      }
    ]
  }
}
```

### Result

If the API succeeded, the `result` key contains the data the end user is interested in.
This could be an object or a list.

### Error

If the API somehow errored, the `error` key contains _at least_:

1. An error message that can be displayed to the end user, under the `error.message` key
2. An error code that can be used to look up translations of error messages, under the `error.code` key
3. A (maybe empty) list of field validation errors. These errors are produced when validating incoming
   requests. Each field validation error contains the same information as the top level `error` key,
   but it also has a `field` key that says exactly which field failed validation.
