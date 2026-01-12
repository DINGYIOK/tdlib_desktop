<script setup lang="ts">
// import { defineProps, defineEmits } from 'vue';

const props = defineProps({
    // 控制弹窗显示
    show: { type: Boolean, default: false },
    // 可选：弹窗标题
    title: { type: String, default: '弹窗标题' },
});

const emit = defineEmits(['update:show']);
// @click="store.huiwangUpload"
// 关闭弹窗
const close = () => {
    emit('update:show', false);
};


</script>

<template>
    <div v-if="show" class="fixed inset-0 bg-black/90 bg-opacity-50 flex items-center justify-center z-50"
        @click.self="close">
        <fieldset class="fieldset bg-base-200 border-base-300 rounded-box w-xs border p-4 z-50">
            <legend class="fieldset-legend">{{ title }}</legend>
            <slot></slot>
            <!-- <button class="btn btn-sm btn-error mt-4" @click="close">关闭</button> -->
        </fieldset>
    </div>
</template>
