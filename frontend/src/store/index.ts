import { defineStore } from "pinia";
import { ref } from "vue";
import router from "../router";

import { main } from "../../wailsjs/go/models";
import {
    AccountSendCode,
    AccountConfirm,
    AccountDelete,
    AccountPageItems,
    AccountMessageSSE,

    AccountPrivateMessage,
    AccountPrivateMessageCount,
    AccountSearchPhone,
    AccountSwitch,

    GetAppInfoStatus,
    SetAppInfo,

} from "../../wailsjs/go/main/App";


export interface AccountItems {
    id: number;
    phone: string;
    name: string;
    is_premium: boolean;
    // proxy_url: string;
    is_active: boolean;
    created_at: string;
}

export const useStore = defineStore("store", () => {

    const accountItems = ref<main.AccountItem[]>();
    const accountPrivateCount = ref(0);

    const appInfoStatus = ref(false);

    // 获取用户列表 分页
    const getAccounts = async (page: number = 1, pageSize: number = 10) => {
        const data = await AccountPageItems(page, pageSize);
        accountItems.value = data;
        console.log("searchAccounts data:", data);
    };

    // 用户发送验证码
    const accountSendCode = async (phone: string) => {
        await AccountSendCode(phone);
    };

    // 用户登陆/新增用户 
    const accountLogin = async (phone: string, code: string, password: string) => {
        await AccountConfirm(phone, code, password);
    };

    // 删除用户
    const accountDelete = async (id: number) => {
        await AccountDelete(id);
    };

    // 发送私信
    const accountSendMessage = async (full_text: string, keyword: string, link_url: string, usernames: string[]) => {
        await AccountPrivateMessage(full_text, keyword, link_url, usernames);
    };

    // 获取可发送次数
    const getAccountPrivateCount = async () => {
        accountPrivateCount.value = await AccountPrivateMessageCount();
    };

    // 根据手机号搜索
    const searchPhoneAccount = async (phone: string) => {
        await AccountSearchPhone(phone);
    };
    // 触发登陆
    const triggerLogin = async (phone: string) => {
        await AccountSwitch(phone);
    };

    // 设置 appID和appHash
    const setAppInfoInit = async (appID: string, appHash: string) => {
        console.log("appID:", appID);
        console.log("appHash:", appHash);
        await SetAppInfo(appID, appHash);
    };
    // 是否设置 appID和appHash
    const getAppInfoStatus = async (): Promise<boolean> => {
        return await GetAppInfoStatus();
    };

    // 初始化
    const appInit = async () => {
        const status = await getAppInfoStatus();
        if (!status) {
            router.push("/login");
        } else {
            router.push("/");
        }

    };


    return {
        accountItems,
        accountPrivateCount,
        searchPhoneAccount,
        getAccountPrivateCount,
        accountSendMessage,
        accountSendCode,
        accountLogin,
        getAccounts,
        triggerLogin,
        accountDelete,
        setAppInfoInit,
        getAppInfoStatus,
        appInit,
    };

});
