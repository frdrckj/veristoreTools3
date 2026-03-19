## COMMON API

### Vendor selection list

**Description**

Vendor selection list API, allowing third-party systems to obtain vendor selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/vendor/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"44EF73D24EF5CF392121B61799FD16170BF8B980CA9B6E1E7895796270A1D3E7",
    "timestamp":1655086148768
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description |
| :---- | :----- | :------- | :---------- |
| label | String | true     | vendor code |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "label":"Verifone"
        },
        {
            "label":"Test"
        }
    ],
    "signature":"1F50D82A9885391A13CC956C30D363325A1B5567E257613C55157497379D10E3"
}
```

### Model selection list

**Description**

Model selection list API, allow third-party systems to obtain model selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| vendor    | String | false    | vendor code |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/model/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"3BE39FE3F47FBFE327D7B534DB2B4C5C4B847C471617FA3D266F95F7B3FB1D17",
    "vendor":"Verifone",
    "timestamp":1655086232224
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description |
| :---- | :----- | :------- | :---------- |
| label | String | true     | model name  |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "label":"X990 v1"
        },
        {
            "label":"X970"
        },
        {
            "label":"X990 v2"
        },
        {
            "label":"X990 v4"
        }
    ],
    "signature":"7F112DE552F7A5E34B22ED5642D458641CC8FD5CDB9C5EA4197FDD90632FF7E0"
}
```

### Timezone selection list

**Description**

Timezone selection list API, allow third-party systems to obtain timezone selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/timeZone/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"1B5A54083FD744F798E9037BF453489B95CCBED377296035152CB3A001251A02",
    "timestamp":1655086064918
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description |
| :---- | :----- | :------- | :---------- |
| label | String | true     | timezone    |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "label":"UTC+12"
        },
        {
            "label":"UTC+11"
        },
        {
            "label":"UTC+10"
        },
        {
            "label":"UTC+9"
        },
        {
            "label":"UTC+8"
        },
        {
            "label":"UTC+7"
        },
        {
            "label":"UTC+6"
        },
        {
            "label":"UTC+5"
        },
        {
            "label":"UTC+4"
        },
        {
            "label":"UTC+3"
        },
        {
            "label":"UTC+2"
        },
        {
            "label":"UTC+1"
        },
        {
            "label":"UTC+0"
        },
        {
            "label":"UTC-1"
        },
        {
            "label":"UTC-2"
        },
        {
            "label":"UTC-3"
        },
        {
            "label":"UTC-4"
        },
        {
            "label":"UTC-5"
        },
        {
            "label":"UTC-6"
        },
        {
            "label":"UTC-7"
        },
        {
            "label":"UTC-8"
        },
        {
            "label":"UTC-9"
        },
        {
            "label":"UTC-10"
        },
        {
            "label":"UTC-11"
        }
    ],
    "signature":"3612C33A8FEE12F8395F85B4D837D328D3733B31F3A58582A52A80B05BB5BC8E"
}
```

### Country selection list

**Description**

Country selection list API, allow third-party systems to obtain country selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/country/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"9CBDBBB8D73197E2D55FC7E2BE532480D29096D51B6477829A4E3238AB823755",
    "timestamp":1655086431733
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description  |
| :---- | :----- | :------- | :----------- |
| id    | String | true     | country ID   |
| label | String | true     | country name |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "id":"1",
            "label":"中国"
        },
        {
            "id":"3",
            "label":"Thailand"
        },
        {
            "id":"4",
            "label":"Nigeria"
        },
        {
            "id":"5",
            "label":"Indonesia"
        },
        {
            "id":"6",
            "label":"Ghana"
        },
        {
            "id":"7",
            "label":"Malaysia"
        },
        {
            "id":"8",
            "label":"Sudan"
        },
        {
            "id":"9",
            "label":"Benin"
        },
        {
            "id":"10",
            "label":"Burkina Faso"
        },
        {
            "id":"11",
            "label":"Cameroon"
        },
        {
            "id":"12",
            "label":"Congo Br"
        },
        {
            "id":"13",
            "label":"Gabon"
        },
        {
            "id":"14",
            "label":"Chad"
        },
        {
            "id":"15",
            "label":"Congo DR"
        },
        {
            "id":"16",
            "label":"Cote D Ivoire"
        },
        {
            "id":"17",
            "label":"Guinea"
        },
        {
            "id":"18",
            "label":"Kenya"
        },
        {
            "id":"19",
            "label":"Liberia"
        },
        {
            "id":"20",
            "label":"Mali"
        },
        {
            "id":"21",
            "label":"Mozambique"
        },
        {
            "id":"22",
            "label":"Senegal"
        },
        {
            "id":"23",
            "label":"Sierra Leone"
        },
        {
            "id":"24",
            "label":"Tanzania"
        },
        {
            "id":"25",
            "label":"Uganda"
        },
        {
            "id":"26",
            "label":"Zambia"
        },
        {
            "id":"27",
            "label":"Singapore"
        },
        {
            "id":"28",
            "label":"Zimbabwe"
        },
        {
            "id":"29",
            "label":"Cambodia"
        }
    ],
    "signature":"5A0A8C6FFA0FD42742D9C1FDA962A6E41017D1F87AD8A9804994CAD1A3D1D6D1"
}
```

### State selection list

**Description**

State selection list API, allow third-party systems to obtain state selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| id        | String | false    | country ID  |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/state/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"CCF867826DA20C35DD50732B95132B64E7883BE8CB87E7936903F0B8827D276C",
    "id":25,
    "timestamp":1655086835189
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description   |
| :---- | :----- | :------- | :------------ |
| id    | String | true     | province ID   |
| label | String | true     | province name |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "id":"509",
            "label":"Adjumani"
        },
        {
            "id":"510",
            "label":"Apac"
        },
        {
            "id":"511",
            "label":"Arua"
        },
        {
            "id":"512",
            "label":"Bugiri"
        },
        {
            "id":"513",
            "label":"Bundibugyo"
        },
        {
            "id":"514",
            "label":"Bushenyi"
        },
        {
            "id":"515",
            "label":"Busia"
        },
        {
            "id":"516",
            "label":"Gulu"
        },
        {
            "id":"517",
            "label":"Hoima"
        },
        {
            "id":"518",
            "label":"Iganga"
        },
        {
            "id":"519",
            "label":"Jinja"
        },
        {
            "id":"520",
            "label":"Kabale"
        },
        {
            "id":"521",
            "label":"Kabarole"
        },
        {
            "id":"522",
            "label":"Kaberamaido"
        },
        {
            "id":"523",
            "label":"Kalangala"
        },
        {
            "id":"524",
            "label":"Kampala"
        },
        {
            "id":"525",
            "label":"Kamuli"
        },
        {
            "id":"526",
            "label":"Kamwenge"
        },
        {
            "id":"527",
            "label":"Kanungu"
        },
        {
            "id":"528",
            "label":"Kapchorwa"
        },
        {
            "id":"529",
            "label":"Kasese"
        },
        {
            "id":"530",
            "label":"Katakwi"
        },
        {
            "id":"531",
            "label":"Kayunga"
        },
        {
            "id":"532",
            "label":"Kibale"
        },
        {
            "id":"533",
            "label":"Kiboga"
        },
        {
            "id":"534",
            "label":"Kisoro"
        },
        {
            "id":"535",
            "label":"Kitgum"
        },
        {
            "id":"536",
            "label":"Kotido"
        },
        {
            "id":"537",
            "label":"Kumi"
        },
        {
            "id":"538",
            "label":"Kyenjojo"
        },
        {
            "id":"539",
            "label":"Lake Albert"
        },
        {
            "id":"540",
            "label":"Lake Victoria"
        },
        {
            "id":"541",
            "label":"Lira"
        },
        {
            "id":"542",
            "label":"Luwero"
        },
        {
            "id":"543",
            "label":"Masaka"
        },
        {
            "id":"544",
            "label":"Masindi"
        },
        {
            "id":"545",
            "label":"Mayuge"
        },
        {
            "id":"546",
            "label":"Mbale"
        },
        {
            "id":"547",
            "label":"Mbarara"
        },
        {
            "id":"548",
            "label":"Moroto"
        },
        {
            "id":"549",
            "label":"Moyo"
        },
        {
            "id":"550",
            "label":"Mpigi"
        },
        {
            "id":"551",
            "label":"Mubende"
        },
        {
            "id":"552",
            "label":"Mukono"
        },
        {
            "id":"553",
            "label":"Nakapiripirit"
        },
        {
            "id":"554",
            "label":"Nakasongola"
        },
        {
            "id":"555",
            "label":"Nebbi"
        },
        {
            "id":"556",
            "label":"Ntungamo"
        },
        {
            "id":"557",
            "label":"Pader"
        },
        {
            "id":"558",
            "label":"Pallisa"
        },
        {
            "id":"559",
            "label":"Rakai"
        },
        {
            "id":"560",
            "label":"Rukungiri"
        },
        {
            "id":"561",
            "label":"Sembabule"
        },
        {
            "id":"562",
            "label":"Sironko"
        },
        {
            "id":"563",
            "label":"Soroti"
        },
        {
            "id":"564",
            "label":"Tororo"
        },
        {
            "id":"565",
            "label":"Wakiso"
        },
        {
            "id":"566",
            "label":"Yumbe"
        }
    ],
    "signature":"41A9E508D9DDDF7CAE209BA7B03EE8B326C31400889A9FD79C663FBF756632D9"
}
```

### City selection list

**Description**

City selection list API, allow third-party systems to obtain city selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| id        | String | false    | province ID |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/city/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"0B08B8367247709C25DEFA6D8468566CA80AA6B05F67BF485AB114C6A4F80392",
    "id":509,
    "timestamp":1655087076053
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description |
| :---- | :----- | :------- | :---------- |
| id    | String | true     | city ID     |
| label | String | true     | city name   |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "id":"4744",
            "label":"East Moyo"
        }
    ],
    "signature":"08196493F67BDD04A19C06FB7A71BF04056609EC36A343093DEAF251F24AA77C"
}
```

### District selection list

**Description**

District selection list API, allow third-party systems to obtain district selection lists

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| id        | String | false    | city ID     |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/common/district/selector
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "signature":"BB0C8F298A93D956BB63F3BBECEDC42FA0FBC160E172BC5A86E9F9F11A5C2EFF",
    "id":4744,
    "timestamp":1655087200533
}
```

**Response Parameters**

| Name  | Type   | Nullable | Description   |
| :---- | :----- | :------- | :------------ |
| id    | String | true     | district ID   |
| label | String | true     | district name |


**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":[
        {
            "id":"25908",
            "label":"Adjumani Tc"
        },
        {
            "id":"25909",
            "label":"Adropi"
        },
        {
            "id":"25910",
            "label":"Ciforo"
        },
        {
            "id":"25911",
            "label":"Dzaipi"
        },
        {
            "id":"25912",
            "label":"Ofua"
        },
        {
            "id":"25913",
            "label":"Pakelle"
        }
    ],
    "signature":"F73E5A2DEBD28A65F0FF7B68BE9BBF4FA715FC10ACD279EEC95912CA18812C49"
}
```
