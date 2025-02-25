**URL**: `/invitations/create`
**方法**: `POST`
**Request Body**:
```json
{
  "inviter_id": 12345,
  "mobile_phone": "+123456789",
  "email": "test@example.com",
  "network": "192.168.1.0/24"
}
```
**Response**:
```json
{
  "success": true,
  "invitation_id": 67890
}
```

#### (2) 响应邀请
```markdown
**URL**: `/invitations/respond`
**方法**: `POST`
**Request Body**:
```json
{
  "invitation_id": 67890,
  "status": "accept" // 或 "reject"
}
```
**Response**:
```json
{
  "success": true,
  "message": "Invitation status updated"
}
```
---

## Access 准入模块设计

### 1. 核心功能
- **准入验证**：
  - 受邀用户通过接受邀请后，需要绑定具体的信息（如网络访问凭证、用户帐号、IP等）。
  - 验证身份证明与邀请记录的一致性。
  
- **网络关联**：
  - 受邀用户接受邀请后（Accept），将其账户与相应网络 `Network` 绑定，实现访问资源的能力。

- **拒绝准入**：
  - 更新 Invitation 状态为 Rejected，并限制访问。

---

### 2. 验证逻辑
当用户通过邀请注册后，可以根据以下逻辑验证：
- 确认 `AcceptStatus == Accept`。
- 判断 `Network` 是否符合要求，下发网络访问权限。

---

### 3. API接口设计
#### (1) 获取准入信息
```markdown
**URL**: `/access/invitation-check`
**方法**: `GET`
**参数**:
  - `invitation_id`: 邀请记录 ID
  - `user_id`: 当前注册用户 ID

**Response**:
```json
{
  "success": true,
  "network": "192.168.1.0/24",
  "access": true
}
```

#### (2) 更改准入状态
```markdown
**URL**: `/access/update-status`
**方法**: `POST`
**Request Body**:
```json
{
  "user_id": 12345,
  "network": "192.168.1.0/24",
  "access_status": "allow" // 或 "deny"
}
```
**Response**:
```json
{
  "success": true,
  "message": "Access status updated"
}
```

---

## 开发范围总结
1. 数据库表结构和模型已基本齐全，无需显著改动。
2. 需要增加：
   - **创建邀请的后端逻辑**：生成邀请码/记录，同时发送邀请。
   - **接受邀请的处理逻辑**：完成受邀用户注册，并更新状态。
   - **准入验证**：结合网络信息，确保用户在网络内具备对应权限。
3. 可扩展功能：
   - 增加超时过期机制（未响应的邀请可设置为超时）。
   - 实现邀请链接加密，如 `/invitation/respond?token=<unique_code>`。

能否提供开发流程的问题或其他需求呢？