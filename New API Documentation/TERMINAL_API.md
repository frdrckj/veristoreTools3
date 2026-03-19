## Terminal API

### List terminal
**Description**

Search Terminal API, which allows third-party systems to search terminal pages.

**Request Parameters**

| Name      | Type    | Nullable | Description                             |
| :-------- | :------ | :------- | :-------------------------------------- |
| accessKey | String  | false    |                                         |
| timestamp | Long    | false    | timestamp                               |
| signature | String  | false    | signature                               |
| page      | Integer | true     | current page of the list<br/> default:1 |
| size      | Integer | true     | rows in a page<br/> default:10          |
| search    | String  | true     | by terminal SN                          |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/terminal/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "search":"YJ",
    "size":10,
    "accessKey":"FA1D66ED",
    "signature":"C9782B09916E47C1657DA0549AE77B6E5C019CA0D31B4074FFB62B75051BA5AE",
    "page":1,
    "timestamp":1656296073945
}
```

**Response Parameters**

| Name         | Type    | Nullable | Description                                    |
| :----------- | :------ | :------- | :--------------------------------------------- |
| id           | Long    | true     | id of terminal                                 |
| sn           | String  | true     | terminal's sequence number                     |
| deviceId     | String  | true     | id of terminal                                 |
| model        | String  | true     | terminal model                                 |
| merchantName | String  | true     | name of the merchant that terminal belongs to  |
| alertStatus  | Integer | true     | alert status<br/> 0：abnormal，<br />1：normal |
| alertMsg     | String  | true     | alert message                                  |

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
                "id":"1541243287471194114",
                "sn":"YJ20220003",
                "deviceId":"",
                "model":"X990 v4",
                "merchantName":"test",
                "alertStatus":0,
                "alertMsg":"Inactive"
            },
            {
                "id":"1541243257075073026",
                "sn":"YJ20220002",
                "deviceId":"",
                "model":"X990 v4",
                "merchantName":"test3",
                "alertStatus":0,
                "alertMsg":"Inactive"
            },
            {
                "id":"1541232441479200769",
                "sn":"YJ20220001",
                "deviceId":"",
                "model":"X990 v4",
                "merchantName":"test_1",
                "alertStatus":0,
                "alertMsg":"Inactive"
            }
        ]
    },
    "signature":"51A3366F8AD81D814B96861B9BF95EDFD5EA34640F492FC7172E6112F55DC59B"
}
```

### Terminal detail

**Description**

Terminal details API, allow third-party systems to obtain terminal details.

**Request Parameters**

| Name       | Type   | Nullable | Description    |
| :--------- | :----- | :------- | :------------- |
| accessKey  | String | false    |                |
| timestamp  | Long   | false    | timestamp      |
| signature  | String | false    | signature      |
| terminalId | String | false    | id of terminal |

**Request Example**

```
POST /url
HTTP/1.1
host:/v1/tps/terminal/detail
content-type:application/json; charset=utf-8;Accept-Language=en-GB;

{
    "accessKey":"0E32C1BC",
    "timestamp":"1656551928493",
    "signature":"9b0c2760ff9e7f79b2ee522592694bc9d7fe39f53cb151b5b2271783f506c0b2",
    "terminalId":"1536249830474330113"
}
```

**Response Parameters**

| Name            | Type         | Nullable | Description                                       |
| :-------------- | :----------- | :------- | :------------------------------------------------ |
| id              | String       | true     | id of terminal                                    |
| sn              | String       | true     | terminal's sequence number                        |
| deviceId        | String       | true     | id of terminal                                    |
| model           | String       | true     | terminal model                                    |
| terminalIcon    | String       | true     | uri of the terminal icon                          |
| vendor          | String       | true     | terminal vendor                                   |
| merchantId      | String       | true     | id of the merchant that terminal belongs to       |
| merchantName    | String       | true     | name of the merchant that terminal belongs to     |
| merchantContact | String       | true     | contact                                           |
| merchantEmail   | String       | true     | contact's email                                   |
| status          | Integer      | true     | terminal status<br/> 0：normal<br/> 1：deactivate |
| alertStatus     | Integer      | true     | alert status<br/> 0：abnormal<br/> 1：normal      |
| alertMsg        | String       | true     | alert message                                     |
| battery         | String       | true     | battery capacity                                  |
| memoryUsage     | String       | true     | usage of memory                                   |
| flashUsage      | String       | true     | usage of flash                                    |
| iotFlag         | Integer      | true     | IOT enable flag<br/> 0：disabled<br/> 1：enabled  |
| iotOnlineFlag   | String       | true     | IOT online flag<br/> 0：offline<br/> 1：online    |
| activeTime      | String       | true     | recentest online time                            |
| groupIds        | List<Long>   | true     | list of groups' ids                               |
| groupNames      | List<String> | true     | list of groups' names                             |
| appInstalls     | List<Object> | true     | list of installed applications                    |
| diagnostic      | List<Object> | true     | list of terminal's diagnostic                     |
| lat             | String       | true     | latitude of terminal's position                   |
| lng             | String       | true     | longitude of terminal's position                  |

*Structure of appInstalls*

| Name        | Type   | Nullable | Description                |
| :---------- | :----- | :------- | :------------------------- |
| appName     | String | false    | application name           |
| installTime | String | false    | installation time          |
| packageName | String | false    | application's package name |
| version     | String | false    | application version        |

*Structure of diagnostic*

| Name      | Type   | Nullable | Description          |
| :-------- | :----- | :------- | :------------------- |
| attribute | String | false    | diagnostic attribute |
| value     | String | false    | diagnostic value     |

**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "id":"1536249830474330113",
        "sn":"00000504V510001544",
        "deviceId":"d1544",
        "model":"X990 v4",
        "terminalIcon":"http://121.41.13.18:8280/system/opt/store/manage/upload/model/picture/52960e27-1bf6-45d6-abc8-ca6246f8bac4.png",
        "vendor":"Verifone",
        "merchantId":"1491601085086625794",
        "merchantName":"test1",
        "merchantContact":"1",
        "merchantEmail":"469604590@qq.com",
        "status":0,
        "alertStatus":1,
        "alertMsg":"Online",
        "battery":" 61",
        "memoryUsage":"44.00",
        "flashUsage":"53.00",
        "iotFlag":1,
        "iotOnlineFlag":0,
        "activeTime":"1656538697000",
        "groupIds":[
            "1536638633303113729",
            "1536885222152200194"
        ],
        "groupNames":[
            "g4",
            "w1"
        ],
        "appInstalls":[
            {
                "appName":"VeriStore",
                "packageName":"cn.verifone.veristore",
                "version":"2.1.0.5",
                "installTime":"1655106323000"
            },
            {
                "appName":"木莲庄酒店",
                "packageName":"com.icbc.gz.mlzhotel",
                "version":"1.0.2",
                "installTime":"1655106323000"
            },
            {
                "appName":"Persistence sample",
                "packageName":"com.verifonce.api",
                "version":"1.8.2",
                "installTime":"1656319642000"
            },
            {
                "appName":"DSTestClient",
                "packageName":"com.verifone.service_demo",
                "version":"2.3.0.10",
                "installTime":"1655106323000"
            },
            {
                "appName":"开机自检测试工具",
                "packageName":"com.verifone.tools.startsverificationtester",
                "version":"1.0.8",
                "installTime":"1656471073000"
            },
            {
                "appName":"Bay Payment",
                "packageName":"com.vfi.android.payment.bay",
                "version":"2.3.17.99",
                "installTime":"1655972852000"
            },
            {
                "appName":"KBXPayment",
                "packageName":"com.vfi.android.payment.kbank",
                "version":"1.6.52",
                "installTime":"1655967392000"
            },
            {
                "appName":"VFService",
                "packageName":"com.vfi.smartpos.deviceservice",
                "version":"3.11.5.0001",
                "installTime":"1655106323000"
            },
            {
                "appName":"VFSystemService",
                "packageName":"com.vfi.smartpos.system_service",
                "version":"1.11.2",
                "installTime":"1655106323000"
            }
        ],
        "diagnostic":[
            {
                "attribute":"PN",
                "value":"M550-104-21-APU-5"
            },
            {
                "attribute":"IMEI",
                "value":"864029040291449"
            },
            {
                "attribute":"MEID",
                "value":""
            },
            {
                "attribute":"Location",
                "value":"119.253322,26.103652"
            },
            {
                "attribute":"Language",
                "value":"en"
            },
            {
                "attribute":"Time Zone",
                "value":"GMT+08:00 : Etc/GMT-8"
            },
            {
                "attribute":"Total Flash",
                "value":"8000000000"
            },
            {
                "attribute":"ROM Version",
                "value":"3A.1.123(202111151040 INTL)"
            },
            {
                "attribute":"Secure Driver Version",
                "value":"VA.213.S.070"
            },
            {
                "attribute":"Total RAM",
                "value":"933232640"
            },
            {
                "attribute":"Mobile Number",
                "value":""
            },
            {
                "attribute":"Available Flash",
                "value":"3786928128"
            },
            {
                "attribute":"Android Version",
                "value":"10"
            },
            {
                "attribute":"Available RAM",
                "value":"518230016"
            },
            {
                "attribute":"Security Chip System Version",
                "value":"X990F_V0.70 (Aug 12 2021 18:25:43)"
            },
            {
                "attribute":"Mobile Network Type",
                "value":""
            },
            {
                "attribute":"Security Chip Protocol Version",
                "value":"X990-V2.1.3(Aug 24 2021 18:01:26)"
            },
            {
                "attribute":"Total mobile data flow",
                "value":"4202622"
            },
            {
                "attribute":"Mobile Ip Address",
                "value":""
            },
            {
                "attribute":"Mobile Network Signal",
                "value":""
            },
            {
                "attribute":"Mobile Monthly Data Flow",
                "value":""
            },
            {
                "attribute":"Network Operator Name",
                "value":""
            },
            {
                "attribute":"Wifi Ip Address",
                "value":"192.168.50.10"
            },
            {
                "attribute":"Wifi Mac Address",
                "value":"E4:08:E7:99:9C:01"
            },
            {
                "attribute":"Lan MAC Address",
                "value":""
            },
            {
                "attribute":"Lan IP Address",
                "value":""
            },
            {
                "attribute":"Power Cycle Times",
                "value":"2"
            },
            {
                "attribute":"Length of printing",
                "value":"0"
            },
            {
                "attribute":"Magnetic Card Swipe times",
                "value":"0"
            },
            {
                "attribute":"Contact Card insert times",
                "value":"0"
            },
            {
                "attribute":"Contactless Card Tap Times",
                "value":"0"
            },
            {
                "attribute":"Front Camera Opening Times",
                "value":"0"
            },
            {
                "attribute":"Rear Camera Opening Times",
                "value":"0"
            },
            {
                "attribute":"Battery Temperature",
                "value":"24.5°"
            },
            {
                "attribute":"Battery Percentage",
                "value":" 61"
            },
            {
                "attribute":"Charging Cycles",
                "value":"0"
            },
            {
                "attribute":"MainBoard Battery Voltage",
                "value":"3.22"
            },
            {
                "attribute":"Uptime",
                "value":"06h00m53s"
            },
            {
                "attribute":"Total Uptime",
                "value":""
            },
            {
                "attribute":"Pci Reboot Time",
                "value":"0400"
            },
            {
                "attribute":"Printer Darkness",
                "value":"0"
            },
            {
                "attribute":"Lock Flag",
                "value":"UnLock"
            },
            {
                "attribute":"Allow Upload Location Info",
                "value":"Allow"
            },
            {
                "attribute":"Msr",
                "value":"Open"
            },
            {
                "attribute":"Icc",
                "value":"Open"
            },
            {
                "attribute":"Contactless",
                "value":"Open"
            },
            {
                "attribute":"Printer",
                "value":"Open"
            },
            {
                "attribute":"Camera",
                "value":"Open"
            },
            {
                "attribute":"Bluetooth",
                "value":"Open"
            },
            {
                "attribute":"Wifi",
                "value":"Open"
            },
            {
                "attribute":"Networks",
                "value":"Close"
            },
            {
                "attribute":"Usb",
                "value":"Open"
            },
            {
                "attribute":"Screen Lock",
                "value":"Close"
            }
        ],
        "lat":"26.103389",
        "lng":"119.252702"
    },
    "signature":"0BB23B1C300BE799B198F3907130F04B613B0382C839BBA19BF68F35A7956B9F"
}
```

### Create terminal

**Description**

Create Terminal API, allow third-party systems to create new terminals.

**Request Parameters**

| Name       | Type    | Nullable | Description                                      |
| :--------- | :------ | :------- | :----------------------------------------------- |
| accessKey  | String  | false    |                                                  |
| timestamp  | Long    | false    | timestamp                                        |
| signature  | String  | false    | signature                                        |
| merchantId | String  | false    | id of the merchant that terminal belongs to      |
| model      | String  | false    | terminal model                                   |
| iotFlag    | Integer | false    | IOT enable flag<br/> 0：disabled<br/> 1：enabled |
| deviceId   | String  | true     | id of terminal                                   |
| groupIds   | List    | true     | list of groups' ids                              |
| sn         | String  | true     | terminal's sequence number                       |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/terminal/add
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "deviceId":"T004",
    "groupIds":[
        1529759931655073793,
        1529759949355036700
    ],
    "iotFlag":0,
    "merchantId":1528986193342836738,
    "model":"X990 v4",
    "signature":"16EE7482F1651B1340A90B91828CDF191FC60DEBD60EA0F9A7A694641D9F1609",
    "sn":"YJ20220004",
    "timestamp":1656552904322
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| resellerId | String | true     | reseller ID                              |
| marketId   | String | true     | id of market that the terminal belong to |
| id         | String | true     | id of terminal                           |
| sn         | String | true     | terminal's sequence number               |
| deviceId   | String | true     | id of terminal                           |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1542320791187378178",
        "marketId":"1491660125548376065",
        "resellerId":"1",
        "sn":"YJ20220004",
        "deviceId":"T004"
    },
    "signature":"279738A08DCC4940CCBFD2A636BEA00529786C1F6D75BB67042756E0B3100E54"
}
```

### Update terminal

**Description**

Update the terminal API, allow third-party systems to update the basic information of the terminal.

**Request Parameters**

| Name       | Type    | Nullable | Description                                     |
| :--------- | :------ | :------- | :---------------------------------------------- |
| accessKey  | String  | false    |                                                 |
| timestamp  | Long    | false    | timestamp                                       |
| signature  | String  | false    | signature                                       |
| deviceId   | String  | ture     | id of terminal                                  |
| groupIds   | List    | true     | list of groups' ids                             |
| id         | String  | false    | id of terminal                                  |
| iotFlag    | Integer | false    | IOT enable flag<br/>0：disabled<br/>1：enabled  |
| merchantId | String  | false    | id of the merchant that terminal belongs to     |
| model      | String  | false    | terminal model                                  |
| status     | Integer | false    | terminal status<br/>0：normal<br/>1：deactivate |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/terminal/update
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "deviceId":"T004-1",
    "groupIds":[
        1529759931655073793
    ],
    "id":1542320791187378178,
    "merchantId":1528986193342836738,
    "model":"X990 v2",
    "signature":"94B36CF1156116DA386E253758CBC57D8EC3AE5A10E36C8CACD39A613900C8CC",
    "status":1,
    "timestamp":1656553530037
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| resellerId | String | true     | reseller ID                              |
| marketId   | String | true     | id of market that the terminal belong to |
| id         | String | true     | id of terminal                           |
| sn         | String | true     | terminal's sequence number               |
| deviceId   | String | true     | id of terminal                           |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1542320791187378178",
        "marketId":"1491660125548376065",
        "resellerId":"1",
        "sn":"YJ20220004",
        "deviceId":"T004-1"
    },
    "signature":"52039267A5FD1C1B9081D0AA79FE90FB3B6DE56CEA17DC5AA2099A50825A2824"
}
```

### Delete terminal

**Description**

Delete terminal API, allow third-party systems to delete terminal.

**Request Parameters**

| Name      | Type   | Nullable | Description   |
| :-------- | :----- | :------- | :------------ |
| accessKey | String | false    |               |
| timestamp | Long   | false    | timestamp     |
| signature | String | false    | signature     |
| id        | String | false    | terminal's id |

**Request Example**

```
POST /url
HTTP/1.1
host:/v1/tps/terminal/delete
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "signature":"11943AA7FE3FB842D3CF82C912DDA3E04A64F71D3DDD6D9E81803F7121CA217D",
    "id":"1542320791187378178",
    "timestamp":1656553795510
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| resellerId | String | true     | reseller ID                              |
| marketId   | String | true     | id of market that the terminal belong to |
| id         | String | true     | id of terminal                           |
| sn         | String | true     | terminal's sequence number               |
| deviceId   | String | true     | id of terminal                           |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "id":"1542320791187378178",
        "marketId":"1491660125548376065",
        "resellerId":"1",
        "sn":"YJ20220004",
        "deviceId":"T004-1"
    },
    "signature":"52039267A5FD1C1B9081D0AA79FE90FB3B6DE56CEA17DC5AA2099A50825A2824"
}
```

### List of terminal applications

**Description**

Search the terminal application API, allowing third-party systems to obtain terminal applications.
**Request Parameters**

| Name       | Type   | Nullable | Description    |
| :--------- | :----- | :------- | :------------- |
| accessKey  | String | false    |                |
| timestamp  | Long   | false    | timestamp      |
| signature  | String | false    | signature      |
| terminalId | String | false    | id of terminal |

**Request Example**

```
POST /url
HTTP/1.1
host: /v2/tps/terminalApp/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
	"accessKey":"1989346D",
	"signature":"2a3718d4130635d1605144e6e9c1850a5033ae2f94836ecda5cbaad4bad28443",
	"terminalId":1827977105384673281,
	"timestamp":1724810012610
}
```

**Response Parameters**

| Name        | Type   | Nullable | Description                |
| :---------- | :----- | :------- | :------------------------- |
| packageName | String | true     | package name               |
| appName     | String | true     | application name           |
| itemList    | List   | true     | list of application detail |

*application detail*

| Name                | Type   | Nullable | Description                                                  |
| :------------------ | :----- | :------- | :----------------------------------------------------------- |
| appIcon             | String | true     | application icon's uri                                       |
| appId               | String | true     | application ID                                               |
| appName             | String | true     | application name                                             |
| appSize             | String | true     | application size                                             |
| appVersion          | String | true     | application version                                          |
| appUsedTemplateName | String | true     | the name of the application parameter template used by the terminal |

**Response Example**

```
{
	"code":"200",
	"data":[
		{
			"itemList":[
				{
					"appIcon":"http://fzjftest.vfcnserv.com:28280/market/opt/store/tempDir/75b3c08d-8c6f-4576-935c-b6ecaa33a81b.png",
					"appId":"1827908599662252034",
					"appName":"VeriStore",
					"appSize":"10.85MB",
					"appUsedTemplateName":"20240828090446_individuation-param.xml",
					"appVersion":"2.5.2"
				}
			],
			"packageName":"cn.verifone.veristore"
		}
	],
	"desc":"Query was successful",
	"signature":"47D5433A2E1A26091DF70CBF060EED74C6CE2F2E7887A9D1445AE45AF2BEE11B"
}
```

### List of terminal application parameter

**Description**

Search the terminal parameter list API, allowing third-party systems to obtain the terminal parameter list.

**Request Parameters**

| Name       | Type   | Nullable | Description     |
| :--------- | :----- | :------- | :-------------- |
| accessKey  | String | false    |                 |
| timestamp  | Long   | false    | timestamp       |
| signature  | String | false    | signature       |
| terminalId | String | false    | id of terminal  |
| appId      | String | false    | application id |

**Request Example**

```
POST /url
HTTP/1.1
host: /v2/tps/terminalAppParameter/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
	"accessKey": "0E32C1BC",
	"timestamp": "1656552802118",
	"signature": "1ff0617726ebfc19546a7238bed0b590b4bb655f0686ec8c35914f20a148b465",
	"terminalId": "1541985304102838274",
	"appId": "1537006258189312001"
}
```

**Response Parameters**

| Name     | Type              | Nullable | Description                                         |
| :------- | :---------------- | :------- | :-------------------------------------------------- |
| paramMap | Map<String, List> | true     | key:parameter set's name， value：parameters' values |

*List*

| Name        | Type    | Nullable | Description                                   |
| :---------- | :------ | :------- | :-------------------------------------------- |
| key         | String  | true     | parameter name                                |
| value       | String  | true     | parameter value                                |
| description | String  | true     | description                                   |
| fixLength   | Integer | true     | fixed length flag<br/> 0:no fixed<br/> 1:fixed |
| maxLength   | Integer | true     | maximum length<br/> 0:not limited             |
value
**Response Example**

```
{
    "code":"200",
    "desc":null,
    "data":{
        "paramMap":{
            "DEFAULT_USER_ACCOUNT":[
                {
                    "key":"TP-DEFAULT_USER_ACCOUNT-DEFAULT_USER_ACCOUNT-1",
                    "value":"0001",
                    "description":"test",
                    "fixLength":0,
                    "maxLength":1000
                }
            ],
            "OPERATOR_INFO":[
                {
                    "key":"TP-OPERATOR_INFO-ACCOUNT_MANAGER_PWD-1",
                    "value":"83743663",
                    "description":"Admin account",
                    "fixLength":0,
                    "maxLength":8
                },
                {
                    "key":"TP-OPERATOR_INFO-SETTING_MANAGER_PWD-1",
                    "value":"333333",
                    "description":"Password for enter setting \nscreen",
                    "fixLength":0,
                    "maxLength":6
                },
                {
                    "key":"TP-OPERATOR_INFO-TRANS_MANAGER_PWD-1",
                    "value":"222222",
                    "description":"Password for sensitive \ntrans",
                    "fixLength":0,
                    "maxLength":6
                }
            ]
        }
    },
    "signature":"3C32520F6B75E5A7C8AF20D2CCA4990C8537FA514FAD1AEB5EA0BA16A132C7D6"
}
```

### Update terminal application parameter

**Description**

Update terminal parameters API, allowing third-party systems to obtain update terminal parameters.

**Request Parameters**

| Name        | Type                | Nullable | Description         |
| :---------- | :------------------ | :------- | :------------------ |
| accessKey   | String              | false    |                     |
| timestamp   | Long                | false    | timestamp           |
| signature   | String              | false    | signature           |
| terminalId  | String              | false    | id of terminal      |
| appId       | String              | false    | application ID      |
| updParamMap | Map<String, String> | false    | modified parameters |

**Request Example**

```
POST /url
HTTP/1.1
host: /v2/tps/terminalAppParameter/update
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"0E32C1BC",
    "timestamp":"1656554158434",
    "signature":"6349d0503f36a3eadc263f0c6bffa174e6fab111e607d724f5c989b4154259e5",
    "terminalId":"1541985304102838274",
    "appId":"1537006258189312001",
    "updParamMap":{
        "TP-OPERATOR_INFO-TRANS_MANAGER_PWD-1":"8888"
    }
}
```

**Response Parameters**

| Name       | Type   | Nullable | Description                              |
| :--------- | :----- | :------- | :--------------------------------------- |
| resellerId | String | true     | reseller ID                              |
| marketId   | String | true     | id of market that the terminal belong to |
| terminalId | String | true     | id of terminal                           |
| appId      | String | true     | application ID                           |
| sn         | String | true     | terminal's sequence number               |
| deviceId   | String | true     | id of terminal                           |

**Response Example**

```
{
    "code":"200",
    "desc":"Operation successful",
    "data":{
        "marketId":"1491600869637812225",
        "resellerId":"1",
        "terminalId":"1541985304102838274",
        "appId":"1537006258189312001",
        "sn":"CS20220002",
        "deviceId":""
    },
    "signature":"D7E3EBC07A64D2C85B7CB12F5D394BBB8BD521FD800582283A3B666F5E4F4B2A"
}
```

