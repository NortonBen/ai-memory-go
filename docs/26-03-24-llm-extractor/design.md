# LLM Extractor Design

## Kiến trúc
`extractor` package nhận Document chunks, gọi API LLM, deserialize JSON trả về các `schema.Node`, `schema.Edge`.

## Interface
`ExtractEntities(text string) ([]Node, []Edge, error)`