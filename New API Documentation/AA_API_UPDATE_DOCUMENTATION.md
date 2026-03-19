# API Update Documentation

## Version Information

- **Version：** v2
- **Release Date：**2024-08-26

## Overview

+ This version of the interface needs to work with VeriStore 2.6.0

## API Updates

### Modified APIs

#### 1、application detail

**URL Updated:**

- **Old URL:** `POST /v1/tps/app/detail`
- **New URL:** `POST /v2/tps/app/detail`

**Request Parameters Updated:**

- **Added Parameter**：
  - `appId`: A new required parameter added to identify the application ID.

**Previous Parameters:**

| Name        | Type   | Nullable | Description         |
| :---------- | :----- | :------- | :------------------ |
| accessKey   | String | false    |                     |
| timestamp   | Long   | false    | timestamp           |
| signature   | String | false    | signature           |
| packageName | String | false    | package name        |
| version     | String | false    | application version |

**Updated Parameters:**

| Name        | Type       | Nullable  | Description         |
| :---------- | :--------- | :-------- | :------------------ |
| accessKey   | String     | false     |                     |
| timestamp   | Long       | false     | timestamp           |
| signature   | String     | false     | signature           |
| packageName | String     | false     | package name        |
| version     | String     | false     | application version |
| **appId**   | **String** | **false** | **application ID**  |

#### 2、List of terminal applications

**URL Updated:**

- **Old URL:** `POST /v1/tps/terminalApp/list`
- **New URL:** `POST /v2/tps/terminalApp/list`

**Response Parameters Updated:**

- **Added Parameter**：
  - `appName`: A new parameter was added to identify the name of the application.
  - `appUsedTemplateName`: A new parameter was added to identify the name of the application parameter template used by the terminal.
  

**Previous Parameters:**

| Name        | Type   | Nullable | Description                |
| :---------- | :----- | :------- | :------------------------- |
| packageName | String | true     | package name               |
| itemList    | List   | true     | list of application detail |

*application detail*

| Name       | Type   | Nullable | Description            |
| :--------- | :----- | :------- | :--------------------- |
| appIcon    | String | true     | application icon's uri |
| appId      | String | true     | application ID         |
| appName    | String | true     | application name       |
| appSize    | String | true     | application size       |
| appVersion | String | true     | application version    |

**Updated Parameters:**

| Name        | Type       | Nullable | Description                |
| :---------- | :--------- | :------- | :------------------------- |
| packageName | String     | true     | package name               |
| **appName** | **String** | **true** | **application name**       |
| itemList    | List       | true     | list of application detail |

*application detail*

| Name                    | Type       | Nullable | Description                                                  |
| :---------------------- | :--------- | :------- | :----------------------------------------------------------- |
| appIcon                 | String     | true     | application icon's uri                                       |
| appId                   | String     | true     | application ID                                               |
| appName                 | String     | true     | application name                                             |
| appSize                 | String     | true     | application size                                             |
| appVersion              | String     | true     | application version                                          |
| **appUsedTemplateName** | **String** | **true** | **the name of the application parameter template used by the terminal** |

#### 3、List of terminal application parameter

**URL Updated:**

- **Old URL:** `POST /v1/tps/terminalAppParameter/list`
- **New URL:** `POST /v2/tps/terminalAppParameter/list`

#### 4、Update terminal application parameter

**URL Updated:**

- **Old URL:** `POST /v1/tps/terminalAppParameter/update`
- **New URL:** `POST /v2/tps/terminalAppParameter/update`

#### 5、Create an application download task

**URL Updated:**

- **Old URL:** `POST /v1/tps/task/appDownload/add`
- **New URL:** `POST /v2/tps/task/appDownload/add`

**Request Parameters Updated:**

- **Deleted Parameter**：
  - `taskSoftwareList.templateId`

**Previous Parameters:**

*taskSoftwareList*

| Name          | Type    | Nullable | Description                                                  |
| :------------ | :------ | :------- | :----------------------------------------------------------- |
| index         | Integer | false    | install in the order                                         |
| launcherFlag  | Integer | false    | launcher flag<br/> 0:no<br/> 1:yes                           |
| softwareId    | String  | false    | software ID                                                  |
| softwareType  | Integer | false    | type<br/> 0:APP                                              |
| templateId    | String  | false    | template ID                                                  |
| uninstallFlag | Integer | false    | whether to uninstall before installing<br/>0:no;<br/> 1:yes; |

**Updated Parameters:**

*taskSoftwareList*

| Name          | Type    | Nullable | Description                                                  |
| :------------ | :------ | :------- | :----------------------------------------------------------- |
| index         | Integer | false    | install in the order                                         |
| launcherFlag  | Integer | false    | launcher flag<br/> 0:no<br/> 1:yes                           |
| softwareId    | String  | false    | software ID                                                  |
| softwareType  | Integer | false    | type<br/> 0:APP                                              |
| uninstallFlag | Integer | false    | whether to uninstall before installing<br/>0:no;<br/> 1:yes; |
