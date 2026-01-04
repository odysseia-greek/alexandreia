# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [aristarchos.proto](#aristarchos-proto)
    - [AggregatorCreationRequest](#olympia_aristarchos-AggregatorCreationRequest)
    - [AggregatorCreationResponse](#olympia_aristarchos-AggregatorCreationResponse)
    - [AggregatorRequest](#olympia_aristarchos-AggregatorRequest)
    - [AggregatorStreamResponse](#olympia_aristarchos-AggregatorStreamResponse)
    - [FormsResponse](#olympia_aristarchos-FormsResponse)
    - [GrammaticalCategory](#olympia_aristarchos-GrammaticalCategory)
    - [GrammaticalForm](#olympia_aristarchos-GrammaticalForm)
    - [HealthRequest](#olympia_aristarchos-HealthRequest)
    - [HealthResponse](#olympia_aristarchos-HealthResponse)
    - [RootWordResponse](#olympia_aristarchos-RootWordResponse)
    - [SearchWordResponse](#olympia_aristarchos-SearchWordResponse)
  
    - [PartOfSpeech](#olympia_aristarchos-PartOfSpeech)
  
    - [Aristarchos](#olympia_aristarchos-Aristarchos)
  
- [Scalar Value Types](#scalar-value-types)



<a name="aristarchos-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## aristarchos.proto



<a name="olympia_aristarchos-AggregatorCreationRequest"></a>

### AggregatorCreationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| word | [string](#string) |  |  |
| rule | [string](#string) |  |  |
| root_word | [string](#string) |  |  |
| translation | [string](#string) |  |  |
| part_of_speech | [PartOfSpeech](#olympia_aristarchos-PartOfSpeech) |  |  |
| trace_id | [string](#string) |  |  |






<a name="olympia_aristarchos-AggregatorCreationResponse"></a>

### AggregatorCreationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| created | [bool](#bool) |  |  |
| updated | [bool](#bool) |  |  |






<a name="olympia_aristarchos-AggregatorRequest"></a>

### AggregatorRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| root_word | [string](#string) |  |  |






<a name="olympia_aristarchos-AggregatorStreamResponse"></a>

### AggregatorStreamResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ack | [string](#string) |  |  |






<a name="olympia_aristarchos-FormsResponse"></a>

### FormsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| word | [string](#string) |  |  |
| unaccented_word | [string](#string) |  |  |
| rule | [string](#string) |  |  |
| root_word | [string](#string) |  |  |
| translation | [string](#string) | repeated |  |
| part_of_speech | [string](#string) |  |  |
| variants | [string](#string) | repeated |  |






<a name="olympia_aristarchos-GrammaticalCategory"></a>

### GrammaticalCategory



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| forms | [GrammaticalForm](#olympia_aristarchos-GrammaticalForm) | repeated |  |






<a name="olympia_aristarchos-GrammaticalForm"></a>

### GrammaticalForm



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| word | [string](#string) |  |  |
| rule | [string](#string) |  |  |






<a name="olympia_aristarchos-HealthRequest"></a>

### HealthRequest







<a name="olympia_aristarchos-HealthResponse"></a>

### HealthResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| health | [bool](#bool) |  |  |






<a name="olympia_aristarchos-RootWordResponse"></a>

### RootWordResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rootWord | [string](#string) |  |  |
| part_of_speech | [PartOfSpeech](#olympia_aristarchos-PartOfSpeech) |  |  |
| translations | [string](#string) | repeated |  |
| categories | [GrammaticalCategory](#olympia_aristarchos-GrammaticalCategory) | repeated |  |
| unaccented_word | [string](#string) |  |  |
| variants | [string](#string) | repeated |  |






<a name="olympia_aristarchos-SearchWordResponse"></a>

### SearchWordResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| word | [string](#string) | repeated |  |





 


<a name="olympia_aristarchos-PartOfSpeech"></a>

### PartOfSpeech


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN_CATEGORY | 0 |  |
| VERB | 1 |  |
| NOUN | 2 |  |
| PARTICIPLE | 3 |  |
| PREPOSITION | 4 |  |
| PARTICLE | 5 |  |
| ADVERB | 6 |  |
| CONJUNCTION | 7 |  |
| ARTICLE | 8 |  |
| PRONOUN | 9 |  |


 

 


<a name="olympia_aristarchos-Aristarchos"></a>

### Aristarchos


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateNewEntry | [AggregatorCreationRequest](#olympia_aristarchos-AggregatorCreationRequest) stream | [AggregatorStreamResponse](#olympia_aristarchos-AggregatorStreamResponse) |  |
| RetrieveEntry | [AggregatorRequest](#olympia_aristarchos-AggregatorRequest) | [RootWordResponse](#olympia_aristarchos-RootWordResponse) |  |
| RetrieveRootFromGrammarForm | [AggregatorRequest](#olympia_aristarchos-AggregatorRequest) | [FormsResponse](#olympia_aristarchos-FormsResponse) |  |
| RetrieveSearchWords | [AggregatorRequest](#olympia_aristarchos-AggregatorRequest) | [SearchWordResponse](#olympia_aristarchos-SearchWordResponse) |  |
| Health | [HealthRequest](#olympia_aristarchos-HealthRequest) | [HealthResponse](#olympia_aristarchos-HealthResponse) |  |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

