import type { RequestRecord } from './requestRecord'

export const SAMPLE_JSON_API = `{
  "jsonapi": { "version": "1.0" },
  "links": {
    "self": "http://example.com/articles",
    "next": "http://example.com/articles?page[offset]=2",
    "last": "http://example.com/articles?page[offset]=10"
  },
  "data": [
    {
      "type": "articles",
      "id": "1",
      "attributes": {
        "title": "JSON:API paints my bikeshed!"
      },
      "relationships": {
        "author": {
          "links": {
            "self": "http://example.com/articles/1/relationships/author",
            "related": "http://example.com/articles/1/author"
          },
          "data": { "type": "people", "id": "9" }
        },
        "comments": {
          "links": {
            "self": "http://example.com/articles/1/relationships/comments",
            "related": "http://example.com/articles/1/comments"
          },
          "data": [
            { "type": "comments", "id": "5" },
            { "type": "comments", "id": "12" }
          ]
        }
      }
    }
  ],
  "included": [
    {
      "type": "people",
      "id": "9",
      "attributes": {
        "first-name": "Dan",
        "last-name": "Gebhardt",
        "twitter": "dgeb"
      },
      "links": { "self": "http://example.com/people/9" }
    },
    {
      "type": "comments",
      "id": "5",
      "attributes": { "body": "First!" },
      "relationships": {
        "author": { "data": { "type": "people", "id": "2" } }
      }
    },
    {
      "type": "comments",
      "id": "12",
      "attributes": { "body": "I like XML better" },
      "relationships": {
        "author": { "data": { "type": "people", "id": "9" } }
      }
    }
  ]
}`

export function buildSampleRecord(): Omit<RequestRecord, 'id' | 'startedAt'> {
  return {
    method: 'GET',
    url: 'http://example.com/articles?include=author,comments',
    requestHeaders: { Accept: 'application/vnd.api+json' },
    requestBody: '',
    status: 200,
    statusText: '200 OK',
    responseHeaders: { 'Content-Type': 'application/vnd.api+json' },
    responseBody: SAMPLE_JSON_API,
    durationMs: 128,
    contentType: 'application/vnd.api+json',
    source: 'manual',
  }
}
