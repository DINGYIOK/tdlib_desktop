<script setup lang="ts">
import { ref } from 'vue';
import { useStore } from "../store";
const props = defineProps<{ show: boolean; }>();

const emit = defineEmits<{ (e: 'update:show', value: boolean): void; }>();
const close = () => {
    phone.value = "";
    verification_code.value = "";
    password.value = "";
    sendCodeBtnText.value = "发送验证码";
    loginBtnText.value = "登陆";
    sendCodeStatus.value = false;
    loginStatus.value = false;
    emit('update:show', false);
};

const store = useStore();
const phone = ref("");
const verification_code = ref("");
const password = ref("");


const sendCodeBtnText = ref("发送验证码");
const sendCodeStatus = ref(false); // 发送验证码的按钮状态

const loginBtnText = ref("登陆");
const loginStatus = ref(false);


// 发送验证码
const snedCode = async () => {
    if (!phone.value) {
        sendCodeBtnText.value = "发送失败❌";
        return;
    }
    try {
        await store.accountSendCode(phone.value.replace(/\s+/g, ''));
        sendCodeStatus.value = true;
        sendCodeBtnText.value = "发送成功✅";
    } catch (err) {
        sendCodeBtnText.value = "发送失败❌";
        return;
    }
};

// 确认登陆
const login = async () => {
    if (!phone.value || !verification_code.value || !password.value) {
        loginBtnText.value = "登陆失败❌";
        return;
    }
    try {
        await store.accountLogin(phone.value.replace(/\s+/g, ''), verification_code.value, password.value);
        loginStatus.value = true;
        loginBtnText.value = "登陆成功✅";
        await store.getAccounts();
        setTimeout(() => { // 登陆成功后1秒后关闭
            close();
        }, 1000);
    } catch (err) {
        loginBtnText.value = "登陆失败❌";
        return;
    }
};


// 登陆按钮的按钮状态
</script>

<template>
    <div v-if="show" class="hero p-10 fixed inset-0 bg-black/90 bg-opacity-50 flex items-center justify-center z-50">
        <div class="hero-content flex-col lg:flex-row-reverse w-full">
            <div class="card bg-base-100 w-full max-w-sm shrink-0 shadow-2xl">
                <div class="card-body">
                    <fieldset class="fieldset">
                        <div class=" flex flex-row">
                            <div>
                                <label class="label">手机号</label>
                                <input v-model="phone" type="text" class="input" placeholder="Phone" />
                            </div>
                            <div class="ml-2"></div>
                            <div class="flex flex-col">
                                <label class="label">&nbsp;</label>
                                <button @click="snedCode()" class="btn btn-secondary  text-xs whitespace-nowrap "
                                    :class="sendCodeStatus ? 'btn-success' : 'btn-secondary'">{{ sendCodeBtnText
                                    }}</button>
                            </div>
                        </div>
                        <div class="mt-2"></div>

                        <label class="label">验证码</label>
                        <input v-model="verification_code" type="text" class="input" placeholder="Verification Code" />
                        <div class="mt-2"></div>

                        <label class="label">二步密码</label>
                        <input v-model="password" type="password" class="input" placeholder="Password" />

                        <button @click="login" class="btn  mt-4"
                            :class="loginStatus ? 'btn-success' : 'btn-secondary'">{{
                                loginBtnText }}</button>
                        <button class="btn btn-error mt-2" @click="close">关闭</button>
                    </fieldset>
                </div>
            </div>
        </div>
    </div>
</template>