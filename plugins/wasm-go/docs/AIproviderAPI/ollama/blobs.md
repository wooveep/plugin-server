# Ollama Blob API

- Source page: https://ollama.readthedocs.io/api/
- Source markdown: https://github.com/ollama/ollama/blob/main/docs/api.md
- Fetched on: 2026-04-23

## Included Sections

- Check if a Blob Exists
- Push a Blob

---

## Check if a Blob Exists

```shell
HEAD /api/blobs/:digest
```

Ensures that the file blob (Binary Large Object) used with create a model exists on the server. This checks your Ollama server and not ollama.com.

### Query Parameters

- `digest`: the SHA256 digest of the blob

### Examples

#### Request

```shell
curl -I http://localhost:11434/api/blobs/sha256:29fdb92e57cf0827ded04ae6461b5931d01fa595843f55d36f5b275a52087dd2
```

#### Response

Return 200 OK if the blob exists, 404 Not Found if it does not.

---

## Push a Blob

```
POST /api/blobs/:digest
```

Push a file to the Ollama server to create a "blob" (Binary Large Object).

### Query Parameters

- `digest`: the expected SHA256 digest of the file

### Examples

#### Request

```shell
curl -T model.gguf -X POST http://localhost:11434/api/blobs/sha256:29fdb92e57cf0827ded04ae6461b5931d01fa595843f55d36f5b275a52087dd2
```

#### Response

Return 201 Created if the blob was successfully created, 400 Bad Request if the digest used is not expected.
