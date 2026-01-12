import { createRouter, createWebHashHistory } from "vue-router";

import Login from "../components/Login.vue";

import Home from "../components/Home.vue";
import ClientAccounts from "../components/ClientAccounts.vue";
import ClientPrivateMessage from "../components/ClientPrivateMessage.vue";

const routes = [
    {
        path: '/',
        component: Home,
        children: [
            // 客户端设置
            {
                path: 'clientAccounts', // 注意，这里不用加斜杠
                component: ClientAccounts
            },
            {
                path: 'clientPrivateMessage', // 注意，这里不用加斜杠
                component: ClientPrivateMessage
            },
        ]
    },
    {
        path: '/login',
        component: Login
    },
];

const router = createRouter({
    history: createWebHashHistory(),
    routes,
});

export default router;