import NProgress from 'nprogress'
import 'nprogress/nprogress.css'

// NProgress 配置
NProgress.configure({
    easing: 'ease',
    speed: 500,
    showSpinner: false, // 是否显示环形加载动画
    trickleSpeed: 200,
    minimum: 0.3 // 初始化时的最小百分比
})

export default NProgress