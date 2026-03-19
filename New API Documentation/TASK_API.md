## Task API

### List tasks

**Description**

Search task API, allowing third-party systems to search task pages.

**Request Parameters**

| Name      | Type    | Nullable | Description                             |
| :-------- | :------ | :------- | :-------------------------------------- |
| accessKey | String  | false    |                                         |
| timestamp | Long    | false    | timestamp                               |
| signature | String  | false    | signature                               |
| page      | Integer | true     | current page of the list<br/> default:1 |
| size      | Integer | true     | rows in a page<br/> default:10          |
| search    | String  | true     | by task's name                          |

**Request Example**

```
POST /task/list
HTTP/1.1
host: /v1/tps/task/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654672095307",
    "signature":"4d0d4c26661adef4046ca39c1217634edcec61488b45bd59572d58fe047115b7",
    "page":1,
    "size":5,
    "search":"test"
}
```

**Response Parameters**

| Name               | Type    | Nullable | Description                                                                                                                              |
| :----------------- | :------ | :------- | :--------------------------------------------------------------------------------------------------------------------------------------- |
| id                 | String  | true     | task ID                                                                                                                                  |
| name               | String  | true     | task name                                                                                                                                |
| type               | Integer | true     | type<br/> 0：to download application<br/> 1：to download application parameter<br/> 2：to uninstall application <br/> 3：to push message |
| terminalNum        | Integer | true     | number of terminals                                                                                                                      |
| downloadPendNum    | Integer | true     | number of terminals that going to download                                                                                               |
| inProgressNum      | Integer | true     | number of terminals that in downloading                                                                                                  |
| downloadSuccessNum | Integer | true     | number of terminals whose is are successful                                                                                              |
| failNum            | Integer | true     | number of terminals whose is are unsuccessful                                                                                            |
| status             | Integer | true     | task status：<br/> 2：canceled <br/> 5：going to download <br/> 6：in downloading <br/> 7、8：finished                                   |

*Structure of taskApps*

**Response Example**

```
{
	"code": "200",
	"desc": "Operation successful",
	"data": {
		"pages": 11,
		"total": 55,
		"list": [
			{
				"id": "1534351381650829314",
				"name": "test_2022060609xyz",
				"type": 1,
				"status": 6,
				"terminalNum": 1,
				"downloadPendNum": 1,
				"inProgressNum": 0,
				"successNum": 0,
				"failNum": 0
			},
			{
				"id": "1534070406467371009",
				"name": "test_2022060607x",
				"type": 1,
				"status": 7,
				"terminalNum": 1,
				"downloadPendNum": 0,
				"inProgressNum": 0,
				"successNum": 1,
				"failNum": 0
			},
			{
				"id": "1534069462014967810",
				"name": "test_2022060711zyz",
				"type": 1,
				"status": 7,
				"terminalNum": 2,
				"downloadPendNum": 2,
				"inProgressNum": 0,
				"successNum": 0,
				"failNum": 0
			},
			{
				"id": "1533988157562593282",
				"name": "test_messagePush_026701",
				"type": 3,
				"status": 7,
				"terminalNum": 1,
				"downloadPendNum": 0,
				"inProgressNum": 0,
				"successNum": 1,
				"failNum": 0
			},
			{
				"id": "1533987164452069378",
				"name": "test_messagePush_0267",
				"type": 3,
				"status": 6,
				"terminalNum": 1,
				"downloadPendNum": 0,
				"inProgressNum": 1,
				"successNum": 0,
				"failNum": 0
			}
		]
	},
	"signature": "31187853233194298051ADF8FA505D75D4954CEFF5017F4E95BF833DAFA8907D"
}
```

### Task detail

**Description**

Task details API, allowing third-party systems to obtain task details.

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| taskId    | String | false    | task ID     |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/detail
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654672294980",
    "signature":"dbe2655a4f06c04f277cd8f18add538cca7598b42f8521050a24ef3dca3e63dc",
    "taskId":"1531112437530267649"
}
```

**Response Parameters**

| Name                       | Type    | Nullable | Description                                                                                                                              |
| :------------------------- | :------ | :------- | :--------------------------------------------------------------------------------------------------------------------------------------- |
| id                         | String  | false    | task ID                                                                                                                                  |
| name                       | String  | false    | task name                                                                                                                                |
| type                       | Integer | false    | type<br/> 0：to download application<br/> 1：to download application parameter<br/> 2：to uninstall application <br/> 3：to push message |
| createTime                 | String  | false    | creation time                                                                                                                            |
| createUserName             | String  | false    | creator                                                                                                                                  |
| terminalNum                | Integer | false    | number of terminals                                                                                                                      |
| downloadPendNum            | Integer | false    | number of terminals that going to download                                                                                               |
| inProgressNum              | Integer | false    | number of terminals that in downloading                                                                                                  |
| successNum                 | Integer | false    | number of terminals whose task is successful                                                                                             |
| failNum                    | Integer | false    | number of terminals whose task is unsuccessful                                                                                           |
| downloadStrategy           | Integer | false    | task strategy，0：begin once online；1：begin in configured time range；                                                                 |
| downloadTime               | String  | false    | time range to begin the task                                                                                                             |
| installModel               | Integer | false    | install or uninstall mode                                                                                                                |
| ,0：silent；1：prompting； |
| installStrategy            | Integer | false    | install strategy,0:install once downloaded;1:install in configured time range;                                                           |
| installTime                | String  | false    | in the time range to install                                                                                                             |
| forceUninstall             | Integer | false    | forced uninstall,0：no，1：yes                                                                                                           |
| taskApps                   | List    | false    | applications related to the task                                                                                                         |

*taskApps*

| Name          | Type    | Nullable | Description                                       |
| :------------ | :------ | :------- | :------------------------------------------------ |
| appApkSize    | String  | false    | size                                              |
| appIcon       | String  | false    | icon's uri                                        |
| appName       | String  | false    | name                                              |
| id            | String  | false    | ID                                                |
| appId         | String  | false    | application ID                                    |
| launcherFlag  | Integer | false    | launcher flag 0:no 1:yes                          |
| softType      | Integer | false    | type                                              |
| uninstallFlag | Integer | false    | whether to uninstall before installing,0:no 1:yes |
| version       | String  | false    | version                                           |

**Response Example**

``` 
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "id":"1531112437530267649",
        "name":"pm_00000504V510001544_05301116",
        "type":3,
        "downloadStrategy":1,
        "downloadTime":"2022-05-31 11:16",
        "installStrategy":0,
        "installTime":"",
        "installModel":null,
        "forceUninstall":null,
        "createUserName":"hlw",
        "createTime":"1653880626000",
        "terminalNum":1,
        "message":"154444",
        "successNum":1,
        "failNum":0,
        "downloadPendNum":0,
        "inProgressNum":0,
        "taskApps":[

        ]
    },
    "signature":"DBB59479455B8271CFC5AAEF2E3C2CBBAF50DBE8118C9570E48A60625AB19962"
}
```

### Task terminal page

**Description**

task terminal page API, which allows third-party systems to search task terminal pages.

**Request Parameters**

| Name      | Type    | Nullable | Description                             |
| :-------- | :------ | :------- | :-------------------------------------- |
| accessKey | String  | false    |                                         |
| timestamp | Long    | false    | timestamp                               |
| page      | Integer | true     | current page of the list<br/> default:1 |
| size      | Integer | true     | rows in a page<br/> default:10          |
| signature | String  | false    | signature                               |
| taskId    | String  | false    | task ID                                 |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/list/terminal
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654672294980",
    "signature":"dbe2655a4f06c04f277cd8f18add538cca7598b42f8521050a24ef3dca3e63dc",
    "taskId":"1531112437530267649"
}
```

**Response Parameters**

| Name         | Type    | Nullable | Description                                  |
| :----------- | :------ | :------- | :------------------------------------------- |
| id           | String  | false    | task terminal ID                             |
| terminalId   | String  | false    | terminal ID                                  |
| taskId       | String  | false    | task ID                                      |
| sn           | String  | false    | terminal's sequence number                   |
| deviceId     | String  | false    | id of terminal                               |
| merchantName | String  | false    | merchant's name                              |
| status       | Integer | false    | terminal task status <br> 0: no start <br> 1: download successful <br> 2: download failed <br> 3: successful installation <br> 4: installation failed <br/> 5: successful delete <br/> 6: delete failed <br/> 7: deactivated <br/> 8: already issued|
| downloadTime | String | false    | application download task : app download time <br>  application parameters download task: application parameters download time    |
| installTime  | String | false    | application download task : application installation time <br>  application parameters download task: application parameters installation time <br> application uninstall task: application uninstallation time <br> |

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
                "id":"1539863585556807684",
                "terminalId":"1536249943896698882",
                "taskId":"1539863585456144385",
                "sn":"YJ20220003",
                "deviceId":"d0164",
                "merchantName":"test1",
                "status":0,
                "downloadTime":"1655967439000",
                "installTime":"1655967559000"
            },
            {
                "id":"1539863585556807683",
                "terminalId":"1536249830474330113",
                "taskId":"1539863585456144385",
                "sn":"YJ20220002",
                "deviceId":"d1544",
                "merchantName":"test2",
                "status":2,
                "downloadTime":"1655967439000",
                "installTime":"1655967499000"
            },
            {
                "id":"1539863585556807682",
                "terminalId":"1536232182940250113",
                "taskId":"1539863585456144385",
                "sn":"YJ20220001",
                "deviceId":"",
                "merchantName":"test3",
                "status":3,
                "downloadTime":"1655967439000",
                "installTime":"1655967559000"
            }
        ]
    },
    "signature":"540E8B53F895C5EE410F2B35A1577FB2292329CC3247323547C21C25818FC040"
}
```

### Create an application download task

**Description**

The Create Application Download Task API allows third-party systems to create new application download tasks.

**Request Parameters**

| Name                | Type         | Nullable | Description                                                                             |
| :------------------ | :----------- | :------- | :-------------------------------------------------------------------------------------- |
| accessKey           | String       | false    |                                                                                         |
| timestamp           | Long         | false    | timestamp                                                                               |
| signature           | String       | false    | signature                                                                               |
| name                | String       | false    | task name                                                                               |
| downloadStrategy    | Integer      | false    | download strategy<br/> 0:begin once online;<br/> 1:begin in configured time range       |
| downloadDateStart   | String       | true     | the date to begin the task<br/> format： yyyy-MM-dd                                     |
| downloadTimeStart   | String       | true     | the time to begin the task<br/> format： hh:mm                                          |
| downloadDateEnd     | String       | true     | the task expiration date<br/> format： yyyy-MM-dd                                       |
| downloadTimeEnd     | String       | true     | the task expiration time<br/> format： hh:mm                                            |
| installMode         | Integer      | false    | install mode<br/> 0：silent<br/> 1：prompting                                           |
| installStrategy     | Integer      | false    | install strategy<br/> 0:install once downloaded<br/> 1:install in configured time range |
| installDateStart    | String       | true     | begin installing after the date<br/> format： yyyy-MM-dd                                |
| installTimeStart    | String       | true     | begin installing after the time<br/> format： hh:mm                                     |
| installTimeEnd      | String       | true     | finish installing before the time<br/> format： hh:mm                                   |
| searchDeviceIdList  | List<String> | true     | search by the list of devices' ids                                                      |
| searchGroupNameList | List<String> | true     | search by the list of groups' names                                                     |
| searchSnList        | List<String> | true     | search by the list of sn                                                                |
| taskSoftwareList    | List<Object> | false    | the list of softwares related to the task                                               |

*taskSoftwareList*

| Name          | Type    | Nullable | Description                                                  |
| :------------ | :------ | :------- | :----------------------------------------------------------- |
| index         | Integer | false    | install in the order                                         |
| launcherFlag  | Integer | false    | launcher flag<br/> 0:no<br/> 1:yes                           |
| softwareId    | String  | false    | software ID                                                  |
| softwareType  | Integer | false    | type<br/> 0:APP                                              |
| uninstallFlag | Integer | false    | whether to uninstall before installing<br/>0:no;<br/> 1:yes; |

**Request Example**

```
POST /url
HTTP/1.1
host: /v2/tps/task/appDownload/add
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654679030038",
    "signature":"c01d5e5c0c44879055876c2b9bb54b2822abda76b6182f5ac21c632b92b9e52e",
    "downloadStrategy":"0",
    "downloadDateStart":"2022-05-12",
    "downloadDateEnd":"2022-06-16",
    "downloadTimeStart":"15:56",
    "downloadTimeEnd":"16:24",
    "installMode":"1",
    "installStrategy":"0",
    "installDateStart":"2022-06-08",
    "installTimeStart":"15:24",
    "installTimeEnd":"16:24",
    "name":"pa_BIJLIPAY_1654679030038",
    "searchSnList":[
        "00000504V510001544"
    ],
    "taskSoftwareList":[
        {
            "index":0,
            "launcherFlag":0,
            "softwareId":"1529749459107131394",
            "softwareType":0,
            "uninstallFlag":0
        }
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
        "id":"1534461179528572929",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"984252DAA1AC15270F42A0D7A20302FFBAA9DFD91178BBA98881A5198D2BAD88"
}
```

### Create an application parameters download task

**Description**

Create application parameter download task API, allowing third-party systems to create new application parameter download tasks.

**Request Parameters**

| Name                | Type         | Nullable | Description                                                                             |
| :------------------ | :----------- | :------- | :-------------------------------------------------------------------------------------- |
| accessKey           | String       | false    |                                                                                         |
| timestamp           | Long         | false    | timestamp                                                                               |
| signature           | String       | false    | signature                                                                               |
| name                | String       | false    | task name                                                                               |
| downloadStrategy    | Integer      | false    | download strategy<br/> 0:begin once online;<br/> 1:begin in configured time range       |
| downloadDateStart   | String       | true     | the date to begin the task<br/> format： yyyy-MM-dd                                     |
| downloadTimeStart   | String       | true     | the time to begin the task<br/> format： hh:mm                                          |
| downloadDateEnd     | String       | true     | the task expiration date<br/> format： yyyy-MM-dd                                       |
| downloadTimeEnd     | String       | true     | the task expiration time<br/> format： hh:mm                                            |
| installStrategy     | Integer      | false    | install strategy<br/> 0:install once downloaded<br/> 1:install in configured time range |
| installDateStart    | String       | true     | begin installing after the date<br/> format： yyyy-MM-dd                                |
| installTimeStart    | String       | true     | begin installing after the time<br/> format： hh:mm                                     |
| installTimeEnd      | String       | true     | finish installing before the time<br/> format： hh:mm                                   |
| searchDeviceIdList  | List<String> | true     | search by the list of devices' ids                                                      |
| searchGroupNameList | List<String> | true     | search by the list of groups' names                                                     |
| searchSnList        | List<String> | true     | search by the list of sn                                                                |
| taskSoftwareList    | List<Object> | false    | the list of softwares related to the task                                               |

*taskSoftwareList*

| Name       | Type   | Nullable | Description |
| :--------- | :----- | :------- | :---------- |
| softwareId | String | false    | software ID |



**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/appParameterDownload/add
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654679787516",
    "signature":"56e367df68a94cd304f6b22edfd233d93a0d44a1d08408bb8a15f0229dbee477",
    "downloadStrategy":"1",
    "downloadTimeStart":"14:40",
    "downloadTimeEnd":"15:40",
    "downloadDateEnd":"2022-06-09",
    "downloadDateStart":"2022-06-09",
    "installStrategy":"1",
    "installDateStart":"2022-06-09",
    "installTimeStart":"16:00",
    "installTimeEnd":"17:00",
    "name":"test_2022060609xyzq",
    "searchSnList":[
        "00000504V510001544"
    ],
    "taskSoftwareList":[
        {
            "softwareId":"1532203368274817026"
        }
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
        "id":"1534464356458000385",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"DBCB70C112B1BA1B9475C7ACD32E08FE1979516842BFCD7CA5CB0E89ADB7788D"
}
```

### Selector of applications related to parameters download task

**Description**

Search for application API that match the task of creating application parameter downloads, allowing third-party systems to search for application pages that match the task of creating application parameter downloads.

**Request Parameters**

| Name                | Type         | Nullable | Description                         |
| :------------------ | :----------- | :------- | :---------------------------------- |
| accessKey           | String       | false    |                                     |
| timestamp           | Long         | false    | timestamp                           |
| signature           | String       | false    | signature                           |
| searchDeviceIdList  | List<String> | true     | search by the list of devices' ids  |
| searchGroupNameList | List<String> | true     | search by the list of groups' names |
| searchSnList        | List<String> | true     | search by the list of sn            |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/appParameterDownload/appList
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654679684774",
    "signature":"4f09b2ba4e2106bb0bf6dba923a4328332ce832cf163fa51c36f6c7cd5f0f1de",
    "searchSnList":[
        "00000504V510001544"
    ]
}
```

**Response Parameters**

| Name        | Type   | Nullable | Description                |
| :---------- | :----- | :------- | :------------------------- |
| packageName | String | false    | package name               |
| itemList    | List   | false    | list of application detail |

*ItemList*

| Name       | Type   | Nullable | Description      |
| :--------- | :----- | :------- | :--------------- |
| appId      | String | false    | application ID   |
| appIcon    | String | false    | icon uri         |
| appName    | String | false    | application name |
| appVersion | String | false    | version          |
| appSize    | String | false    | APK size         |

**Response Example**

```
{
    "code":"200",
    "desc":"Query was successful",
    "data":{
        "apps":[
            {
                "packageName":"cn.verifone.veristore",
                "itemList":[
                    {
                        "appId":"1532203368274817026",
                        "appIcon":"http://121.41.13.18:8280/market/opt/store/tempDir/0eedba10-9c94-417f-92a5-ed33cec6d95f.png",
                        "appName":"VeriStore",
                        "appVersion":"2.1.0.3",
                        "appSize":"13.05MB"
                    }
                ]
            }
        ]
    },
    "signature":"5FC0B82D530F16BF0EB905753BED9DAF49F7B1F2ED69FF2C9898708623CD4352"
}
```

### Create an application uninstall task

**Description**

Create app uninstall task API, allowing third-party systems to create new app uninstall tasks.

**Request Parameters**

| Name                | Type         | Nullable | Description                                                                               |
| :------------------ | :----------- | :------- | :---------------------------------------------------------------------------------------- |
| accessKey           | String       | false    |                                                                                           |
| timestamp           | Long         | false    | timestamp                                                                                 |
| signature           | String       | false    | signature                                                                                 |
| searchDeviceIdList  | List<String> | true     | search by the list of devices' ids                                                        |
| searchGroupNameList | List<String> | true     | search by the list of groups' names                                                       |
| searchSnList        | List<String> | true     | search by the list of sn                                                                  |
| name                | String       | false    | task name                                                                                 |
| forceUninstall      | Integer      | false    | forced uninstall<br/> 0：no<br/> 1：yes                                                   |
| unInstallMode       | Integer      | false    | uninstall mode<br/> 0：silent<br/> 1：prompting                                           |
| uninstallStrategy   | Integer      | false    | uninstall strategy<br/> 0:install once downloaded<br/> 1:install in configured time range |
| uninstallDateStart  | String       | true     | begin uninstalling after the date<br/> format： yyyy-MM-dd                                |
| uninstallTimeStart  | String       | true     | begin installing after the time<br/> format： hh:mm:ss                                    |
| taskSoftwareList    | List<Object> | false    | the list of softwares related to the task                                                 |

*taskSoftwareList*

| Name  | Type    | Nullable | Description            |
| :---- | :------ | :------- | :--------------------- |
| appId | String  | false    | application ID         |
| index | Integer | false    | uninstall in the order |



**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/appUninstall/add
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654680149060",
    "signature":"335ee1b78a518bbf3d54a2419ce737bb8a8f70ef4aff362d5d02e2962fd826df",
    "name":"app_uninstall_24",
    "forceUninstall":"0",
    "searchDeviceIdList":[
        "test_7",
        "test_8",
        "test_9"
    ],
    "searchGroupNameList":[
        "test_29",
        "test_30"
    ],
    "searchSnList":[
        "VQ202200011",
        "VQ202200012",
        "VQ202200013",
        "VQ202200014",
        "VQ202200015",
        "VQ202200016"
    ],
    "softwareList":[
        {
            "appId":"1532179175307046914",
            "index":1
        },
        {
            "appId":"1531896659434106882",
            "index":2
        },
        {
            "appId":"1529749459107131394",
            "index":3
        }
    ],
    "unInstallMode":"1",
    "uninstallStrategy":"1",
    "uninstallDateStart":"2022-06-10",
    "uninstallTimeStart":"17:00:00"
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
        "id":"1534465873164464129",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"AD9C3F1A168C067F605C2B251A825F37F79EA56035574082E9D26995461A9817"
}
```

### Create a message push task

**Description**

Create message push task API, allow third-party systems to create new message push tasks.

**Request Parameters**

| Name                 | Type         | Nullable | Description                                                              |
| :------------------- | :----------- | :------- | :----------------------------------------------------------------------- |
| accessKey            | String       | false    |                                                                          |
| timestamp            | Long         | false    | timestamp                                                                |
| signature            | String       | false    | signature                                                                |
| searchDeviceIdList   | List<String> | true     | search by list of devices' ids                                           |
| searchGroupNameList  | List<String> | true     | search by list of groups' names                                          |
| searchSnList         | List<String> | true     | search by list of sn                                                     |
| name                 | String       | false    | task name                                                                |
| message              | String       | false    | message                                                                  |
| notificationStrategy | Integer      | false    | notify strategy<br/> 0:notify immediately<br/> 1:notify as schedule      |
| appointedDate        | String       | true     | schedule date<br/> format： yyyy-MM-dd                                   |
| appointedTime        | String       | true     | schedule time<br/> format： hh:mm                                        |
| expireStrategy       | Integer      | false    | expiration strategy<br/> 0：never expire<br/> 1：expire after expiration |
| expireDate           | String       | true     | expiration date<br/>format： yyyy-MM-dd                                  |
| expireTime           | String       | true     | expiration time<br/>format： HH:mm                                       |
| pushMessageTo        | String       | false    | message recipient<br/> 0：launch<br/> 1：App                             |
| appId                | String       | true     | message recipient application ID                                         |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/messagePush/add
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "timestamp":"1654680287842",
    "signature":"7bbb3beb76b72ea0045729de84a2332d00a2b743f482cafe35ccdfdb630b6ae3",
    "searchSnList":[
        "VQ202200001"
    ],
    "name":"test_messagePush_008a",
    "message":"test",
    "notificationStrategy":"1",
    "appointedDate":"2022-06-09",
    "appointedTime":"12:30",
    "expireStrategy":"1",
    "expireDate":"2022-06-09",
    "expireTime":"14:30",
    "pushMessageTo":"0"
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
        "id":"1534466454960566274",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"E1F0F64037DBD0EB05187214F6085B1ABED7AC8565E3B223A143BEA53A0C9518"
}
```

### Cancel a task

**Description**

Cancel task API, allow third-party systems to cancel tasks.

**Request Parameters**

| Name      | Type   | Nullable | Description |
| :-------- | :----- | :------- | :---------- |
| accessKey | String | false    |             |
| timestamp | Long   | false    | timestamp   |
| signature | String | false    | signature   |
| taskId    | String | false    | task ID     |

**Request Example**

```
POST /url
HTTP/1.1
host: /v1/tps/task/cancel
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
    "accessKey":"FA1D66ED",
    "signature":"67E148BDE36BB79E7DE3B73D449913BD8C1A5A2E7EFAEC48122717D8AA4924AB",
    "taskId":1534375459862417410,
    "timestamp":1654741882251
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
        "id":"1534375459862417410",
        "marketId":"1491660125548376065",
        "resellerId":"1"
    },
    "signature":"F9CEAB073C87786F2258A438032E635BF17EEF24633F50C9DA0DE5B1FEE3C711"
}
```

