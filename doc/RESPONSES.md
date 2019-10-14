## Standardized API responses

All API responses sent from Teslacoil follow the same format. 


### Result

If the API succeeded, the result of the operation is returned directly. This may be 
an object or a list. 

### Error

If the API somehow errored, there's a `error` key that contains _at least_:

1. An error message that can be displayed to the end user, under the `error.message` key
2. An error code that can be used to look up translations of error messages, under the `error.code` key
3. A (maybe empty) list of field validation errors. These errors are produced when validating incoming
   requests. Each field validation error contains the same information as the top level `error` key,
   but it also has a `field` key that says exactly which field failed validation.

An error response looks like this: 

```json
{
  "error": {
    "message": "this is an error message",
    "code": "ERR_HERE_IS_A_CODE",
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