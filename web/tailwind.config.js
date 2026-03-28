/** @type {import('tailwindcss').Config} */
module.exports = {
    content: ["./index.html", "./src/**/*.{vue,js,ts,jsx,tsx}"],
    theme: {
        extend: {
            // 除非你要自定义 primary 颜色，否则这里保持为空或只放非颜色配置
        },
    },
    plugins: [require("daisyui")], // 必须确保这里引入了 daisyui
    daisyui: {
        themes: ["light", "dark"], // 确保启用了主题
    },
}