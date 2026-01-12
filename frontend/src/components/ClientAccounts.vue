<script setup lang="ts">
import Verification from "./Verification.vue";
import ClientAccountLogin from "./ClientAccountLogin.vue";
// import ClientDialogue from "./ClientDialogue.vue";
import Tips from "./Tips.vue";
import { useStore } from "../store";
import { computed, ref, onMounted, watch } from "vue";
const store = useStore();

const fieldLst = ["ID", "手机号码", "名称", "状态", "会员", "创建时间"];
// 搜索
const keywordValue = ref(""); // 搜索字段值

const showLoginModal = ref(false); // 上传窗口展示

// 翻页
const page = ref(1);  // 页
const pageSize = ref(10); // 页数量

const showCard = ref(false); // 提示窗口状态
const content = ref(""); // 提示窗口文字

// 删除参数
const deleteStatusText = ref("删除中..."); // 删除状态文本
const showDeleteModal = ref(false); // 删除谷歌验证码窗口
const deleteStatus = ref(false); // 删除按钮状态
const deleteValueID = ref(0); // 要删除的ID
const deleteCode = ref("");
const deleteValuePhone = ref("");


// 初始化执行
onMounted(async () => {
    // store
    await store.getAccounts();
    // 触发登陆
    // await store.triggerLogin();
    // 触发机器人
    // await store.triggerStartBot();

});
// 向前翻页
const addTurnPage = async () => {
    const itemsLength = store.accountItems?.length ? store.accountItems?.length : 0;
    if (itemsLength < pageSize.value) {
        // 提示暂无更多
        content.value = "暂无更多数据";
        showCard.value = true;
        return;
    }
    page.value += 1;
    await store.getAccounts(page.value, pageSize.value);
};

// 向后翻页
const subTurnPage = async () => {
    if (page.value == 1) {
        content.value = "已经是第一页了";
        showCard.value = true;
        return;
    }
    page.value -= 1;
    await store.getAccounts(page.value, pageSize.value);
};

// 确认搜索
const handleSearch = async () => {
    await store.searchPhoneAccount(keywordValue.value.replace("+", ""));
};

// 提示并保存要删除的BotID
const handleDeleteValue = (id: number, phone: string) => {
    deleteValuePhone.value = phone;
    deleteValueID.value = id;
    showDeleteModal.value = true;
};

// 确认删除
const handleDelete = async () => {
    deleteStatus.value = true;
    const accountID = deleteValueID.value;
    const phone = deleteValuePhone.value;
    try {
        // 先出发点登陆
        await store.triggerLogin(phone);
        await store.accountDelete(accountID);
        deleteStatusText.value = "删除成功";
    } catch (err) {
        deleteStatusText.value = "删除失败";
    }
    setTimeout(async () => {
        showDeleteModal.value = false;
        deleteStatus.value = false;
        await store.getAccounts(1, 10);
    }, 1500);
};


// 根据删除动态返回按钮样式
const deleteBtnClass = computed(() => {
    switch (deleteStatusText.value) {
        case "删除成功":
            return "btn btn-sm btn-success mt-4";
        case "删除中":
            return "btn btn-sm btn-warning mt-4";
        case "删除失败":
            return "btn btn-sm btn-error mt-4";
        default:
            return "btn btn-sm btn-neutral mt-4";
    }
});

// 搜索字段计算属性
// const keywordMatch = computed(() => {
//     switch (keyword.value) {
//         case "ID":
//             return "id";
//         case "名称":
//             return "bot_name";
//         case "机器人ID":
//             return "bot_id";
//         case "默认金额":
//             return "amount";
//         case "收款地址":
//             return "address";
//         case "启动端口":
//             return "port";
//         case "是否启用":
//             return "is_active";
//     }
// });

// 关闭提示弹窗
const handleClose = () => {
    showCard.value = false;
};

watch([keywordValue], async ([newKeywordValue]) => {
    if (newKeywordValue == "") { // 如果监听的搜素字段值为空则请求
        await store.getAccounts();
    }
})


</script>

<template>
    <Tips v-if="showCard" :content="content" @close="handleClose"></Tips>

    <ClientAccountLogin v-model:show="showLoginModal"></ClientAccountLogin>
    <Verification v-model:show="showDeleteModal" title="请输入谷歌验证码">
        <div class="join">
            <input v-model="deleteCode" type="text" class="input join-item" placeholder="验证码" />
            <button class="btn btn-neutral join-item" @click="handleDelete">确认删除</button>
        </div>
        <button v-show="deleteStatus" :class="deleteBtnClass">{{ deleteStatusText }}</button>
    </Verification>

    <div class="p-5">
        <span class="font-mono text-xl">账号列表</span>
        <div class="flex flex-row justify-between mt-2">
            <div class="flex flex-row">
                <div class="join">
                    <div>
                        <label class="input validator join-item">
                            <svg class="h-[1em] opacity-50" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
                                <g stroke-linejoin="round" stroke-linecap="round" stroke-width="2.5" fill="none"
                                    stroke="currentColor">
                                    <circle cx="11" cy="11" r="8"></circle>
                                    <path d="m21 21-4.3-4.3"></path>
                                </g>
                            </svg>
                            <input type="text" placeholder="搜索的手机号码" required v-model="keywordValue" />
                        </label>
                        <!-- <div class="validator-hint hidden">Enter valid email address</div> -->
                    </div>
                    <button class="btn btn-neutral join-item" @click="handleSearch">搜索</button>
                </div>
            </div>
            <div class="flex flex-row">
                <!-- <input type="file" class="file-input w-50" accept=".txt" @change="handleFileChange" /> -->
                <button class="btn btn-info join-item ml-1" @click=" showLoginModal = true">
                    新增账号</button>
            </div>
        </div>

        <div class="overflow-x-auto mt-5">
            <table class="table table-zebra">
                <!-- head -->
                <thead>
                    <tr>
                        <th v-for="(field, index) in fieldLst" :key="index">{{ field }}</th>
                    </tr>
                </thead>
                <tbody class="text-xs!">
                    <!-- row 1 -->
                    <tr v-for="(item, index) in store.accountItems" :key="index">
                        <th>{{ item.id }}</th>
                        <!-- <td><a :href="`https://t.me/${item.bot_name}`" target="_blank"><button
                                    class="btn btn-xs btn-soft">{{ item.bot_name }}</button></a></td> -->
                        <td>{{ item.phone }}</td>
                        <td>{{ item.name }}</td>
                        <td>{{ item.is_active ? "正常" : "封号" }}</td>
                        <td>{{ item.is_premium ? "✅" : "" }}</td>
                        <!-- <td>{{ item.proxy_url }}</td> -->

                        <td>{{ item.create_at }}</td>
                        <td>
                            <!-- <button @click="showDialogueModal = true" class=" btn btn-xs btn-success">查看</button> -->
                            <button @click="handleDeleteValue(item.id, item.phone)"
                                class="btn btn-xs btn-error ml-2">删除</button>
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>

        <div class="join fixed bottom-0 left-0 right-0 z-10 flex justify-center p-4 shadow-md">

            <button @click="subTurnPage()" class="join-item btn">«</button>
            <button class="join-item btn">Page {{ page }}</button>
            <button @click="addTurnPage()" class="join-item btn">»</button>
        </div>
    </div>
</template>