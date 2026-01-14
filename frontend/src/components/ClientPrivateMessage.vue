<script setup lang="ts">
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

import { computed, onMounted, ref, watch, nextTick, onUnmounted } from 'vue';
import { useStore } from "../store";
const store = useStore();

const accountMessageSSE = ref("");

const btnText = ref("点击发送");

const logTextarea = ref<HTMLTextAreaElement | null>(null);

const messageText = ref(""); // 私信文案
const messageKeyWords = ref(""); // 私信关键词
const messageKeyWordsURL = ref(""); // 私信关键词链接
const messageUsers = ref(""); // 私信用户名列表

const messageUserCount = computed(() => {
    // 和你手写的逻辑完全一致
    if (!messageUsers.value.endsWith('\n')) {
        messageUsers.value += '\n';
    }
    const str2 = "bot";
    return messageUsers.value
        .split('\n')
        .filter(line => line.trim() !== '' && !line.toLowerCase().includes(str2.toLowerCase()))
        .length;
});

// 发送按钮
const start = async () => {
    if (!messageText.value || !messageKeyWords.value || !messageKeyWordsURL.value || !messageUsers.value) {
        console.log("字段为空");
        return;
    }
    // 将用户名拆分为列表
    if (!messageUsers.value.endsWith("\n")) {
        messageUsers.value += "\n";
    }
    const str2 = "bot";
    const users = messageUsers.value
        .split("\n")
        .filter(line => line.trim() !== '' && line.toLowerCase().includes(str2.toLowerCase()));

    if (users.length > store.accountPrivateCount) {
        console.log("超过最大可发送数量");
        return;
    }

    console.log("messageText:", messageText.value);
    console.log("messageKeyWords:", messageKeyWords.value);
    console.log("messageKeyWordsURL:", messageKeyWordsURL.value);
    console.log("users:", users);


    try {
        btnText.value = "开始发送";
        await store.accountSendMessage(messageText.value, messageKeyWords.value, messageKeyWordsURL.value, users);
    } catch (err) {
        console.log("发送私信错误:", err);
        btnText.value = "发送失败❌";
    }
};



onMounted(async () => {
    await store.getAccountPrivateCount();
    EventsOn("private_message", (msg) => {
        accountMessageSSE.value += `${msg}\n`;
    });
});


onUnmounted(() => {
    EventsOff("private_message");
});

watch(
    () => accountMessageSSE.value,
    async () => {
        await nextTick();
        if (logTextarea.value) {
            logTextarea.value.scrollTop = logTextarea.value.scrollHeight;
        }
    }
)

/**
 
--------
专属福利288彩金已下发钱包

打开👉 @fllqb 👈钱包领取福利
--------

https://t.me/ID_fllqbbot
*/




</script>

<template>
    <div class="p-5">
        <span class="font-mono text-xl">一键私信 可私信次数:{{ store.accountPrivateCount }}</span>

        <div class="flex h-screen flex-row justify-between">
            <aside class="w-full p-1 flex flex-col h-full overflow-y-auto ">
                <div class="p-1  w-full flex justify-end rounded-lg">
                    <fieldset class="fieldset w-full xrounded-lg">
                        <legend class="fieldset-legend">粘贴私信用户名 粘贴识别数量:{{ messageUserCount }}</legend>
                        <textarea v-model="messageUsers" class="textarea h-40 w-full" placeholder="用户名..."></textarea>
                        <!-- <div class="label">Optional</div> -->
                    </fieldset>
                </div>
                <div class="divider"></div>
                <div class="p-1  flex justify-end rounded-lg">
                    <fieldset class="fieldset w-full  rounded-lg ">
                        <legend class="fieldset-legend">粘贴私信文案</legend>
                        <textarea v-model="messageText" class="textarea w-full h-24" placeholder="私信文案..."></textarea>
                        <!-- <div class="label">Optional</div> -->
                        <input v-model="messageKeyWords" type="text" placeholder="关键字" class="input w-full " />
                        <input v-model="messageKeyWordsURL" type="text" placeholder="关键字链接" class="input w-full" />
                    </fieldset>
                </div>
                <button @click="start" class="btn btn-info mt-5">{{ btnText }}</button>
            </aside>

            <aside class="w-full p-1 flex flex-col h-full l overflow-y-auto ">
                <div class="p-1   flex justify-end rounded-lg">
                    <fieldset class="fieldset w-full h-full   rounded-lg">
                        <legend class="fieldset-legend">私信发送日志</legend>
                        <textarea ref="logTextarea" class="textarea h-120 w-full text-xs" placeholder="用户日志..."
                            v-model="accountMessageSSE"></textarea>
                    </fieldset>
                </div>
            </aside>


        </div>
    </div>
</template>