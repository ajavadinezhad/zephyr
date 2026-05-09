import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/zyrln/',
  title: 'Zyrln',
  description: 'Domain-fronting relay',
  locales: {
    root: {
      label: 'English',
      lang: 'en',
      link: '/',
      title: 'Zyrln Documentation',
      description: 'Domain-fronting relay to bypass DPI-based censorship',
      themeConfig: {
        nav: [
          { text: 'Home', link: '/' },
          { text: 'Getting Started', link: '/getting-started' },
          { text: 'GitHub', link: 'https://github.com/ajavadinezhad/zyrln' }
        ],
        sidebar: {
          '/': [
            {
              text: 'Quick Start',
              items: [
                { text: 'Introduction', link: '/' },
                { text: 'Getting Started', link: '/getting-started' }
              ]
            },
            {
              text: 'Installation',
              items: [
                { text: 'Desktop Setup', link: '/installation/desktop' },
                { text: 'Android Setup', link: '/installation/android' },
                { text: 'Apps Script Setup', link: '/installation/appsscript' },
                { text: 'VPS Setup', link: '/installation/vps' }
              ]
            },
            {
              text: 'Advanced Guide',
              items: [
                { text: 'Configuration', link: '/configuration' },
                { text: 'Limitations', link: '/limitations' },
                { text: 'Troubleshooting', link: '/troubleshooting' }
              ]
            },
            {
              text: 'Contributing',
              items: [
                { text: 'Contributing Guide', link: '/contributing' }
              ]
            }
          ]
        }
      }
    },
    fa: {
      label: 'فارسی',
      lang: 'fa',
      link: '/fa/',
      dir: 'rtl',
      title: 'مستندات Zyrln',
      description: 'رله Domain-Fronting برای عبور از سانسور مبتنی بر DPI',
      themeConfig: {
        nav: [
          { text: 'خانه', link: '/fa/' },
          { text: 'شروع سریع', link: '/fa/getting-started' },
          { text: 'گیت‌هاب', link: 'https://github.com/ajavadinezhad/zyrln' }
        ],
        sidebar: {
          '/fa/': [
            {
              text: 'شروع سریع',
              items: [
                { text: 'مقدمه', link: '/fa/' },
                { text: 'شروع به کار', link: '/fa/getting-started' }
              ]
            },
            {
              text: 'راه اندازی ورکر',
              items: [
                { text: 'راه‌اندازی در سرور مجازی', link: '/fa/installation/vps-worker' },
                { text: 'راه‌اندازی ورکر کلودفلر', link: '/fa/installation/cloudlfare-worker' }
              ]
            },
            {
              text: 'کلاینت',
              items: [
                { text: 'نصب روی دسکتاپ', link: '/fa/installation/desktop' },
                { text: 'نصب روی اندروید', link: '/fa/installation/android' }
              ]
            },
            {
              text: 'راهنمای پیشرفته',
              items: [
                { text: 'تنظیمات', link: '/fa/configuration' },
                { text: 'محدودیت‌ها', link: '/fa/limitations' },
                { text: 'عیب‌یابی', link: '/fa/troubleshooting' }
              ]
            },
            {
              text: 'مشارکت',
              items: [
                { text: 'راهنمای مشارکت', link: '/fa/contributing' }
              ]
            }
          ]
        }
      }
    }
  },

  themeConfig: {
    logo: '/public/logo.png',
    siteTitle: 'Zyrln',
    socialLinks: [
      { icon: 'github', link: 'https://github.com/ajavadinezhad/zyrln' }
    ],
    footer: {
      message: 'MIT License',
      copyright: 'Copyright © 2024 Ahmad Javadi Nezhad'
    }
  }

})