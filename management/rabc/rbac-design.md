## linkany access policy

## linkany 中的针对用户的rbac
linkany中所有的策略都是针对某一个或者多个分组进行，创建分组的用户为owner， owner拥有对创建的分组的所有权限，包括增删改查策略，owner可以将分组授权给邀请通过的其他用户，被授权的用户可以对分组进行增删改查策略操作，但是不能将分组授权给其他用户 。
对分组的操作权限表示如下:

```shell
	// 分组权限
	PermCreateGroup = "group:create"
	PermDeleteGroup = "group:delete"
	PermUpdateGroup = "group:update"
	PermViewGroup   = "group:view"
```
对节点具体的操作权限如下，在某个授权的分组里，用户可以对节点进行增删改查操作:
```shell
    // 节点权限
    PermAddNode     = "node:add"
    PermRemoveNode  = "node:remove"
    PermUpdateNode  = "node:update"
    PermConnectNode = "node:connect"
```

对邀请的成员的操作权限如下:
```shell
 // 成员权限
    PermManageMembers = "members:manage"
    PermViewMembers   = "members:view"
```

对策略的操作权限如下:
```shell
    // 策略权限
    PermCreatePolicy = "policy:create"
    PermDeletePolicy = "policy:delete"
    PermUpdatePolicy = "policy:update"
    PermViewPolicy   = "policy:view"
```

举例，对邀请的用户zhangsan配置在group "test-network"里，从节点A到节点B的连接策略，那么zhangsan需要有以下权限:
```shell
    // zhangsan权限
    PermViewGroup
    PermViewPolicy
    PermCreatePolicy
    PermUpdatePolicy
    PermDeletePolicy
```
json格式为:
```json
{
    "username": "zhangsan",
    "permissions": [
        "group:view",
        "policy:view",
        "policy:create",
        "policy:update",
        "policy:delete"
    ], 
    "group": "test-network",
    "policy": {
        "source": "nodeA",
        "sourceType": "node",
        "target": "nodeB",
        "targetType": "node",
        "effect": "allow",
        "action": "connect",
        "condition": {
            "time": "2021-09-01T00:00:00Z"
        }
    }
}
```
上边代表的意思是：为用户zhangsan分配了组的查看权限，策略的查看、创建、更新、删除权限，zhangsan在test-network组里，可以查看test-network组的策略，同时zhangsan可以创建、更新、删除策略， 针对"test-network"这个组里的A和B节点，zhangsan可以连接A和B节点，条件是在2021-09-01T00:00:00Z之前。


## 节点分组 + 策略
用户可以创建分组，将节点分别加入到不同的分组，默认情况下，所有节点都属于`default`分组，只有同一个分组内的节点才能互相访问。 

## 组内策略
策略类型有以下几种类型：
- 节点类型： 这种是针对节点做的策略，直接配置源节点和目标节点互通访问或者拒绝访问, 用Effect字段表示(allow/deny)，节点之间action为: connect和disconnect
- 标签类型： 用户针对节点打标签，只要符合策略设置的标签之间的节点才能互相通信
- 时间类型： 用户可以设置时间段，只有在时间段内的节点才能互相通信
通常一个策略可以重复用在多个组里，但是节点类型因为是关联到节点ID，所以只能在一个组里使用。

## 组内Node-Node策略配置
二个节点类型的策略配置，节点A到节点B的连接，在上午0点-12之间可以连接通信
```json
{
    "username": "zhangsan",
    "policy": {
        "source": "nodeA",
        "sourceType": "node",
        "target": "nodeB",
        "targetType": "node",
        "effect": "allow",
        "action": "connect",
        "condition": {
            "start": "2021-09-01T00:00:00Z",
            "end": "2021-09-01T12:00:00Z"
        }
    }
}
```

## 组内Node-Tag策略配置
节点A到标签为`api`的节点可以连接通信
```json
{
    "username": "zhangsan",
    "policy": {
        "source": "nodeA",
        "sourceType": "node",
        "target": "api",
        "targetType": "tag",
        "effect": "allow",
        "action": "connect"
    }
}
```

## 组内Node-Time策略配置
节点A到节点B的连接，在上午0点-12之间可以连接通信
```json
{
    "username": "zhangsan",
    "action:": "policy:create",
    "policy": {
        "source": "nodeA",
        "sourceType": "node",
        "target": "nodeB",
        "targetType": "node",
        "effect": "allow",
        "action": "connect",
        "condition": {
            "start": "2021-09-01T00:00:00Z",
            "end": "2021-09-01T12:00:00Z"
        }
    }
}
```

## 组内Tag-Tag通讯
标签为`api`的节点可以和标签为`db`的节点通讯
```json
{
    "username": "zhangsan",
    "action:": "policy:create",
    "policy": {
        "source": "api",
        "sourceType": "tag",
        "target": "db",
        "targetType": "tag",
        "effect": "allow",
        "action": "connect"
    }
}
```

## 删除策略
标签为`api`的节点可以和标签为`db`的策略删除
```json
{
    "username": "zhangsan",
    "action:": "policy:delete",
    "policy": {
        "id": "policy-id"
    }
}
```