## Group API

### Group list

**Description**

The search grouping API that allows third-party systems to search grouped pages.

**Request Parameters**

| Name      | Type    | Nullable | Description                             |
| :-------- | :------ | :------- | :-------------------------------------- |
| accessKey | String  | false    |                                         |
| timestamp | Long    | false    | timestamp                               |
| signature | String  | false    | signature                               |
| page      | Integer | true     | current page of the list<br/> default:1 |
| size      | Integer | true     | rows in a page<br/> default:10          |
| search    | String  | true     | by group name                           |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/group/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "timestamp":"1654668995758",
    "signature":"b52995ca60eb1e87f524237976319a2adcc600dcfeb48f79d0675962277e07ba",
    "page":1,
    "size":1
}
```

**Response Parameters**

| Name             | Type   | Nullable | Description             |
| :--------------- | :----- | :------- | :---------------------- |
| id               | String | true     | id of group             |
| groupName        | String | true     | group name              |
| totalTerminalNum | String | true     | the number of terminals |

**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "pages":58,
        "total":58,
        "list":[
            {
                "id":"1532253015412404225",
                "groupName":"test_30",
                "totalTerminalNum":2
            }
        ]
    },
    "signature":"E0D5659FA6C63E2B2375F5A9914AC357F405809BEA3442AED871339D75819EEF"
}
```

### Group detail

**Description**

Group details API, allowing third-party systems to obtain group details.

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| groupId   | String | false    | id of group |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/group/detail
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654669109143",
    "signature":"b9512119e159f92026394bf76a01f804ed2be414a383d485cfcbb6fb9c7a636a",
    "groupId":"1530088941723398146"
}
```

**Response Parameters**

| Name        | Type   | Nullable | Description                             |
| :---------- | :----- | :------- | :-------------------------------------- |
| id          | String | true     | id of group                             |
| groupName   | String | true     | group name                              |
| terminalIds | List   | true     | list of terminals' ids exclude subgroup |
| subGroups   | List   | true     | list of subgroups' ids                  |

  *subgroup*

| Name             | Type    | Nullable | Description         |
| :--------------- | :------ | :------- | :------------------ |
| id               | String  | true     | group id            |
| groupName        | String  | true     | group name          |
| totalTerminalNum | Integer | true     | number of terminals |

**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "id":"1530088941723398146",
        "groupName":"test_15",
        "subGroups":[
            {
                "id":"1529764226597203970",
                "groupName":"16",
                "totalTerminalNum":0
            },
            {
                "id":"1529764155801546754",
                "groupName":"12",
                "totalTerminalNum":5
            },
            {
                "id":"1529764213653581825",
                "groupName":"15",
                "totalTerminalNum":4
            },
            {
                "id":"1529764286970015745",
                "groupName":"19",
                "totalTerminalNum":0
            },
            {
                "id":"1529760096256339970",
                "groupName":"11",
                "totalTerminalNum":5
            },
            {
                "id":"1529764197773946881",
                "groupName":"14",
                "totalTerminalNum":3
            },
            {
                "id":"1529764255651147778",
                "groupName":"18",
                "totalTerminalNum":0
            },
            {
                "id":"1529764377525039105",
                "groupName":"20",
                "totalTerminalNum":0
            },
            {
                "id":"1529764239519854593",
                "groupName":"17",
                "totalTerminalNum":0
            },
            {
                "id":"1529764181672013826",
                "groupName":"13",
                "totalTerminalNum":5
            }
        ],
        "terminalIds":[
            "1529999437641625601",
            "1529999437645819905",
            "1529999437658402817",
            "1529999437696151553",
            "1529999437708734465",
            "1529999437712928769",
            "1529999437729705985",
            "1529999437759066113",
            "1529999437775843329",
            "1529999437792620545"
        ]
    },
    "signature":"9ADE14ABF13AAAAB927675DF7AEE52CE7506871696ACC6BFA82A7B6E5E71BFF6"
}
```

### Create group

**Description**

Create Group API, allowing third-party systems to create new groups.

**Request Parameters**

| Name        | Type   | Nullable | Description            |
| :---------- | :----- | :------- | :--------------------- |
| accessKey   | String | false    |                        |
| timestamp   | Long   | false    | timestamp              |
| signature   | String | false    | signature              |
| groupName   | String | false    | group name             |
| subGroupIds | List   | false    | list of subGroups' ids |
| terminalIds | List   | false    | list of terminals' ids |

**Request Example**

```
POST /url
HTTP/1.1
host:/v1/tps/group/add/normal
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654669425513",
    "signature":"76f7eccbcdff9093ef542d09548936666ba77afe26b047f3a3c45be49a05c646",
    "groupName":"test_2022060801",
    "subGroupIds":[
        "1529759931655073793",
        "1529759949355036673",
        "1529759965851234306",
        "1529759997581144065"
    ],
    "terminalIds":[
        "1529999437641625601",
        "1530019462012284930",
        "1530019462012284931",
        "1530019462012284932"
    ]
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description |
| :--------- | :----- | :------- | :---------- |
| id         | String | false    | id          |
| marketId   | String | false    | market ID   |
| resellerId | String | false    | reseller ID |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1534420894958292994",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"87A9CD4623E6A28B147CB61718700C904920ABCF1107BAC6D5B3119E911063B9"
}
```

### Update group

**Description**

Update grouping API, allowing third-party systems to update grouping information.

**Request Parameters**

| Name        | Type   | Nullable | Description            |
| :---------- | :----- | :------- | :--------------------- |
| accessKey   | String | false    |                        |
| timestamp   | Long   | false    | timestamp              |
| signature   | String | false    | signature              |
| id          | String | false    | group ID               |
| groupName   | String | false    | group name             |
| subGroupIds | List   | false    | list of subgroups' ids |
| terminalIds | List   | false    | list of terminals' ids |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/group/update/normal
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654669570880",
    "signature":"78db02f15483b48471d909ccf670000621e0b2d30485a18c7b9e98d4fe56d89e",
    "id":"1531830281297526785",
    "groupName":"test_27",
    "subGroupIds":[
        "1529759997581144065",
        "1529764213653581825"
    ],
    "terminalIds":[
        "1530019462008090625",
        "1530019462008090626",
        "1530019462012284930",
        "1530019462012284931"
    ]
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
        "id":"1531830281297526785",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"8C642B240533F11E1E7C2601A6C8002BB1D64DFF79AF4A39F2BA65F96E6BBC13"
}
```

### Delete group

**Description**

Delete group API, allowing third-party systems to delete groups.

**Request Parameters**
| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| id        | String | false    | group id    |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/group/delete
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654669678335",
    "signature":"cb4a2bf193b600018c495ddf0f4843623f2610f8551a203fa104599447e77681",
    "id":"1534420894958292994"
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
        "id":"1534420894958292994",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"87A9CD4623E6A28B147CB61718700C904920ABCF1107BAC6D5B3119E911063B9"
}
```