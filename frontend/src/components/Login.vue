<script setup lang="ts">
import { ref } from "vue";
import { useStore } from "../store";
import Tips from "./Tips.vue";
import router from "../router";
const store = useStore();

const showCard = ref(false); // 提示窗口状态
const content = ref(""); // 提示窗口文字


const appID = ref("");
const appHash = ref("");

const setAppInfo = async () => {
    if (!appID.value || !appHash.value) {
        handOpen("appID或者appHash不能为空❌");
        return;
    }
    try {
        await store.setAppInfoInit(appID.value, appHash.value);
        router.push("/");
    } catch (err) {
        handOpen(`设置AppID或者AppHash设置错误❌ ${err}`);
    }
};

// 关闭提示弹窗
const handleClose = () => {
    showCard.value = false;
};

const handOpen = (text: string) => {
    showCard.value = true;
    content.value = text;
}

</script>

<template>
    <Tips v-if="showCard" :content="content" @close="handleClose"></Tips>
    <div>
        <div class="hero bg-base-200 min-h-screen">
            <div class="hero-content flex-col lg:flex-row-reverse w-1/3">
                <div class="card bg-base-100 w-full max-w-sm shrink-0 shadow-2xl">
                    <div class="card-body">
                        <fieldset class="fieldset">
                            <label class="label">AppID</label>
                            <input v-model="appID" type="text" class="input" placeholder="请输入AppID" />
                            <label class="label">AppHash</label>
                            <input v-model="appHash" type="password" class="input" placeholder="请输入Hash" />

                            <!-- TODO 添加登陆验证-->
                            <div @click="setAppInfo" class="btn btn-neutral mt-4">设置</div>
                        </fieldset>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>