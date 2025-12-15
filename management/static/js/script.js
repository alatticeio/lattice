class PushManager {
    constructor() {
        this.initEventListeners();
        this.updateCharCount();
    }

    initEventListeners() {
        // 推送按钮
        document.getElementById('pushBtn').addEventListener('click', () => {
            this.handlePush();
        });

        // 批量推送按钮
        document.getElementById('batchPushBtn').addEventListener('click', () => {
            this.openBatchModal();
        });

        // 清空按钮
        document.getElementById('clearBtn').addEventListener('click', () => {
            this.clearForm();
        });

        // 内容输入监听
        document.getElementById('pushContent').addEventListener('input', () => {
            this.updateCharCount();
        });

        // 模态框事件
        this.initModalEvents();
    }

    initModalEvents() {
        const modal = document.getElementById('batchModal');
        const closeBtn = document.querySelector('.close');
        const cancelBtn = document.querySelector('.cancel-batch');
        const addItemBtn = document.getElementById('addBatchItem');
        const confirmBtn = document.getElementById('confirmBatchPush');

        // 打开模态框
        document.getElementById('batchPushBtn').onclick = () => {
            modal.style.display = 'block';
        }

        // 关闭模态框
        const closeModal = () => {
            modal.style.display = 'none';
        }

        closeBtn.onclick = closeModal;
        cancelBtn.onclick = closeModal;

        // 点击模态框外部关闭
        window.onclick = (event) => {
            if (event.target === modal) {
                closeModal();
            }
        }

        // 添加批量项
        addItemBtn.addEventListener('click', () => {
            this.addBatchItem();
        });

        // 确认批量推送
        confirmBtn.addEventListener('click', () => {
            this.handleBatchPush();
        });
    }

    async handlePush() {
        const url = document.getElementById('pushUrl').value.trim();
        const content = document.getElementById('pushContent').value.trim();
        const pushBtn = document.getElementById('pushBtn');
        const btnText = pushBtn.querySelector('.btn-text');
        const btnLoading = pushBtn.querySelector('.btn-loading');

        // 验证输入
        if (!url || !content) {
            this.showResult('请填写推送地址和内容', false);
            return;
        }

        if (content.length > 5000) {
            this.showResult('推送内容不能超过5000字符', false);
            return;
        }

        // 显示加载状态
        btnText.style.display = 'none';
        btnLoading.style.display = 'inline';

        try {
            const response = await fetch('/api/push', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    url: url,
                    content: content
                })
            });

            const result = await response.json();

            if (result.success) {
                this.showResult(`✅ 推送成功！\n\n响应数据:\n${result.data || '无返回数据'}`, true);
            } else {
                this.showResult(`❌ 推送失败: ${result.message}`, false);
            }
        } catch (error) {
            this.showResult(`🚨 网络错误: ${error.message}`, false);
        } finally {
            // 恢复按钮状态
            btnText.style.display = 'inline';
            btnLoading.style.display = 'none';
        }
    }

    async handleBatchPush() {
        const batchItems = document.querySelectorAll('.batch-item');
        const pushes = [];

        // 收集批量推送数据
        batchItems.forEach((item, index) => {
            const url = item.querySelector('.batch-url').value.trim();
            const content = item.querySelector('.batch-content').value.trim();

            if (url && content) {
                pushes.push({
                    url: url,
                    content: content
                });
            }
        });

        if (pushes.length === 0) {
            this.showResult('请至少填写一个有效的推送项', false);
            return;
        }

        try {
            const response = await fetch('/api/push/batch', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    pushes: pushes
                })
            });

            const result = await response.json();

            if (result.success) {
                let resultText = `📦 批量推送完成！\n\n`;
                result.results.forEach(item => {
                    resultText += `项目 ${item.index}: ${item.url}\n`;
                    resultText += `状态: ${item.status === 'success' ? '✅ 成功' : '❌ 失败'}\n`;
                    if (item.error && item.error !== 'null') {
                        resultText += `错误: ${item.error}\n`;
                    }
                    resultText += `---\n`;
                });
                this.showResult(resultText, true);

                // 关闭模态框
                document.getElementById('batchModal').style.display = 'none';
            } else {
                this.showResult(`批量推送失败: ${result.message}`, false);
            }
        } catch (error) {
            this.showResult(`批量推送网络错误: ${error.message}`, false);
        }
    }

    showResult(message, isSuccess) {
        const resultBox = document.getElementById('result');
        resultBox.innerHTML = `<div class="result-content">${message}</div>`;

        resultBox.className = 'result-box';
        if (isSuccess) {
            resultBox.classList.add('result-success');
        } else {
            resultBox.classList.add('result-error');
        }

        // 滚动到结果区域
        resultBox.scrollIntoView({ behavior: 'smooth' });
    }

    clearForm() {
        document.getElementById('pushUrl').value = '';
        document.getElementById('pushContent').value = '';
        this.updateCharCount();
        document.getElementById('result').innerHTML = '<div class="result-placeholder">推送结果将显示在这里...</div>';
        document.getElementById('result').className = 'result-box';
    }

    updateCharCount() {
        const content = document.getElementById('pushContent').value;
        document.getElementById('charCount').textContent = content.length;

        // 字符数警告
        const charCount = document.getElementById('charCount');
        if (content.length > 4500) {
            charCount.style.color = '#f56565';
        } else if (content.length > 4000) {
            charCount.style.color = '#ed8936';
        } else {
            charCount.style.color = '#718096';
        }
    }

    addBatchItem() {
        const batchItems = document.getElementById('batchItems');
        const newItem = document.createElement('div');
        newItem.className = 'batch-item';
        newItem.innerHTML = `
            <input type="url" placeholder="推送地址" class="batch-url">
            <textarea placeholder="推送内容" class="batch-content"></textarea>
            <button type="button" class="btn-remove">删除</button>
        `;

        batchItems.appendChild(newItem);

        // 添加删除事件
        newItem.querySelector('.btn-remove').addEventListener('click', () => {
            newItem.remove();
        });
    }

    openBatchModal() {
        // 清空现有项（除了第一个）
        const batchItems = document.getElementById('batchItems');
        while (batchItems.children.length > 1) {
            batchItems.removeChild(batchItems.lastChild);
        }

        // 清空第一个项的内容
        const firstItem = batchItems.querySelector('.batch-item');
        if (firstItem) {
            firstItem.querySelector('.batch-url').value = '';
            firstItem.querySelector('.batch-content').value = '';
        }

        document.getElementById('batchModal').style.display = 'block';
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    new PushManager();

    // 加载推送历史
    loadPushHistory();
});

// 加载推送历史（示例）
async function loadPushHistory() {
    try {
        const response = await fetch('/api/push/history');
        const result = await response.json();

        if (result.success) {
            console.log('推送历史:', result.data);
        }
    } catch (error) {
        console.log('获取推送历史失败:', error);
    }
}