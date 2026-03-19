## Merchant API

### List merchants

**Description**

The Search Merchant API allows third-party systems to search for merchant pages.

**Request Parameters**

| Name      | Type    | Nullable | Description                             |
| :-------- | :------ | :------- | :-------------------------------------- |
| accessKey | String  | false    |                                         |
| timestamp | Long    | false    | timestamp                               |
| signature | String  | false    | signature                               |
| page      | Integer | true     | current page of the list<br/> default:1 |
| size      | Integer | true     | rows in a page<br/> default:10          |
| search    | String  | true     | by merchant's name                      |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/merchant/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "search":"K",
    "size":3,
    "accessKey":"FA1D66ED",
    "signature":"3D8B37DC114238C2596085697C719EE37961A26AD67ABE95791E005FFEE18DD5",
    "page":1,
    "timestamp":1653459511206
}
```

**Response Parameters**

| Name         | Type   | Nullable | Description              |
| :----------- | :----- | :------- | :----------------------- |
| id           | String | true     | id of merchant           |
| address      | String | true     | merchant's address       |
| cellPhone    | String | true     | merchant's phone number  |
| contact      | String | true     | merchant contact         |
| email        | String | true     | merchant contact's email |
| merchantName | String | true     | merchant's name          |

**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "pages":1,
        "total":3,
        "list":[
            {
                "id":"1534707232521625602",
                "merchantName":"Kampala Hospital",
                "cellPhone":"13107946812",
                "email":"wu.y@verifone.cn",
                "address":"8 Dundas Road, Lower Kololo, Kampala, Uganda",
                "contact":"WuYao"
            },
            {
                "id":"1534469879777501185",
                "merchantName":"Kololo Courts Hotel",
                "cellPhone":"13107946812",
                "email":"bin.y@verifone.cn",
                "address":"3 Dundas Road, Lower Kololo, Kampala, Uganda",
                "contact":"BinYao"
            },
            {
                "id":"1534468307869802498",
                "merchantName":"Kush Lounge",
                "cellPhone":"13107946811",
                "email":"ju.y@verifone.cn",
                "address":"35 John Babiha (Acacia) Ave, Kampala, Uganda",
                "contact":"JuYao"
            }
        ]
    },
    "signature":"94C48B91ADBB683C8C97C4155FA9A098BA3E6B5BC2A863053CA20AD3E9D68BCE"
}
```

### Merchant detail

**Description**

Merchant Details API, which allows third-party systems to obtain merchant details.

**Request Parameters**

| Name       | Type   | Nullable | Description    |
| :--------- | :----- | :------- | :------------- |
| accessKey  | String | false    |                |
| timestamp  | Long   | false    | timestamp      |
| signature  | String | false    | signature      |
| merchantId | String | false    | id of merchant |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/merchant/detail
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "merchantId":1534707232521625602,
    "signature":"79A78F6821A7BAA376B597FC8BA9D09A379499CBCE226676D539852C3931E352",
    "timestamp":1654737780328
}
```

**Response Parameters**

| Name         | Type   | Nullable | Description                              |
| :----------- | :----- | :------- | :--------------------------------------- |
| address      | String | true     | merchant's address                       |
| cellPhone    | String | true     | merchant's phone number                  |
| cityId       | String | true     | id of merchant's city                    |
| cityName     | String | true     | name of merchant's city                  |
| contact      | String | true     | merchant contact                         |
| countryId    | String | true     | id of merchant's country                 |
| countryName  | String | true     | name merchant's country                  |
| districtId   | String | true     | id of merchant's district                |
| districtName | String | true     | name of merchant's district              |
| email        | String | true     | merchant contact's email                 |
| id           | String | true     | id of merchant                           |
| marketId     | String | true     | id of market that the merchant belong to |
| merchantName | String | true     | merchant's name                          |
| postCode     | String | true     | zip code                                 |
| resellerId   | String | true     | id of the merchant's reseller            |
| stateId      | String | true     | id of merchant's state                   |
| stateName    | String | true     | name of merchant's state                 |
| telePhone    | String | true     | telephone                                |
| timeZone     | String | true     | timezone                                 |
| tagList      | List   | true     | tag list                                 |

  *tagList*

| Name | Type   | Nullable | Description |
| :--- | :----- | :------- | :---------- |
| id   | String | true     | tag ID      |
| tag  | String | true     | tag name    |

**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "id":"1534707232521625602",
        "resellerId":"1",
        "marketId":"1491660125548376065",
        "merchantName":"Kampala Hospital",
        "countryId":"25",
        "countryName":"Uganda",
        "stateId":"524",
        "stateName":"Kampala",
        "cityId":"4790",
        "cityName":"Kampala",
        "districtId":"26162",
        "districtName":"Central Division",
        "timeZone":"UTC+3",
        "address":"8 Dundas Road, Lower Kololo, Kampala, Uganda",
        "postCode":null,
        "contact":"WuYao",
        "email":"wu.y@verifone.cn",
        "cellPhone":"13107946812",
        "telePhone":null,
        "tagList":[
            {
                "id":"1534707232563568641",
                "tag":"24h"
            },
            {
                "id":"1534707232571957249",
                "tag":"hospital"
            }
        ]
    },
    "signature":"8DFF533E7A8249FC4C86F41BD3210FC3E1BD8FB70100132A9DB7C6DF02F9C246"
}
```

### Create merchant

**Description**

Create Merchant API that allows third-party systems to create new merchants.

**Request Parameters**

| Name         | Type   | Nullable | Description               |
| :----------- | :----- | :------- | :------------------------ |
| accessKey    | String | false    |                           |
| timestamp    | Long   | false    | timestamp                 |
| signature    | String | false    | signature                 |
| address      | String | false    | merchant's address        |
| cellPhone    | String | false    | merchant's phone number   |
| cityId       | String | false    | id of merchant's city     |
| contact      | String | false    | merchant contact          |
| countryId    | String | false    | id of merchant's country  |
| districtId   | String | true     | id of merchant's district |
| email        | String | false    | merchant contact's email  |
| merchantName | String | false    | merchant's name           |
| postCode     | String | true     | postcode                  |
| stateId      | String | false    | id of merchant's state    |
| tags         | List   | true     | tag,splitted by comma     |
| telePhone    | String | true     | telephone                 |
| timeZone     | String | false    | timezone                  |

*Structure of tagList*

| Type   | Nullable | Description    |
| :----- | :------- | :------------- |
| String | false    | merchant's tag |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/merchant/add
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "address":"8 Dundas Road, Lower Kololo, Kampala, Uganda",
    "signature":"AC639A24EB55FD0F21E1D5A679167D2229E2491C3DBEDA95FCF37BC7276FA88B",
    "stateId":524,
    "timeZone":"UTC+3",
    "cityId":4790,
    "countryId":25,
    "merchantName":"Kampala Hospital",
    "tags":"hospital,24h",
    "districtId":26162,
    "accessKey":"FA1D66ED",
    "contact":"WuYao",
    "cellPhone":"13107946812",
    "email":"wu.y@verifone.cn",
    "timestamp":1654737691485
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| id         | String | false    | id                                       |
| marketId   | String | false    | id of market that the merchant belong to |
| resellerId | String | false    | id of the merchant's reseller            |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1534707232521625602",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"34BFFEF7B9971A40D35218395AE21C54BC15A5C0FCD08A2425F69C760D613539"
}
```

### Update merchant

**Description**

Update Merchant API to allow third-party systems to update merchant information.

**Request Parameters**

| Name         | Type   | Nullable | Description               |
| :----------- | :----- | :------- | :------------------------ |
| accessKey    | String | false    |                           |
| timestamp    | Long   | false    | timestamp                 |
| signature    | String | false    | signature                 |
| address      | String | false    | merchant's address        |
| cellPhone    | String | false    | merchant's phone number   |
| cityId       | String | false    | id of merchant's city     |
| contact      | String | false    | merchant contact          |
| countryId    | String | false    | id of merchant's country  |
| districtId   | String | true     | id of merchant's district |
| id           | String | false    | id of merchant            |
| email        | String | false    | merchant contact's email  |
| merchantName | String | false    | merchant's name           |
| postCode     | String | true     | postcode                  |
| stateId      | String | false    | id of merchant's state    |
| tags         | List   | true     | tags,splitted by comma    |
| telePhone    | String | true     | telephone                 |
| timeZone     | String | true     | timezone                  |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/merchant/update
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654668370846",
    "signature":"52de129d95c0be28d583b196978758e28100495eb5beba14473a812430780b36",
    "address":"Kampala, Uganda",
    "cellPhone":"123456789012",
    "cityId":4790,
    "contact":"LinYao",
    "countryId":25,
    "districtId":26162,
    "email":"lin.y@verifone.cn",
    "merchantName":"ONOMO Hotel Kampala",
    "postCode":"123",
    "stateId":524,
    "telePhone":"1234567890",
    "timeZone":"UTC+3",
    "tags":"24h",
    "id":"1529033427732213762"
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| id         | String | false    | id                                       |
| marketId   | String | false    | id of market that the merchant belong to |
| resellerId | String | false    | id of the merchant's reseller            |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1529033427732213762",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"8B74BF43A8062353D93DA2402A493708A2F6B236D25B8581E076AFDA5EF14DD6"
}
```

### Delete merchant

**Description**

Delete Merchant API to allow third-party systems to delete merchants.

**Request Parameters**
| Name      | Type   | Nullable | Description   |
| :-------- | :----- | :------- | :------------ |
| accessKey | String | false    |               |
| timestamp | Long   | false    | timestamp     |
| signature | String | false    | signature     |
| id        | String | false    | merchant's id |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/merchant/delete
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654668554807",
    "signature":"6ba5eaa087782a2d52b6297605478531e83f3405a13b88ae6ff855171a466125",
    "id":"1534362368282042369"
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| id         | String | false    | id                                       |
| marketId   | String | false    | id of market that the merchant belong to |
| resellerId | String | false    | id of the merchant's reseller            |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1534362368282042369",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"98D7D37E41EEFB7C165E947C29358C23701AE13CDB6CA35EE39FC4A50CB2C08F"
}
```