<script setup lang="ts">

import Sidebar from './Sidebar.vue';
</script>

<template>
    <!-- <Header></Header> -->
    <div class="drawer drawer-open ">
        <input id="my-drawer-4" type="checkbox" class="drawer-toggle" checked />
        <div class="drawer-content">
            <!-- Page content here -->
            <router-view></router-view>
        </div>
        <Sidebar></Sidebar>
    </div>
</template>
<style scoped></style>
