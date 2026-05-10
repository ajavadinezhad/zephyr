document.addEventListener('DOMContentLoaded', () => {
    const translations = {
        en: {
            'app.title': 'Zyrln Control Center',
            'brand.name': 'ZYRLN',
            'button.connect': 'Connect',
            'button.disconnect': 'Disconnect',
            'button.theme': 'Toggle theme',
            'button.close': 'Close',
            'section.config': 'Relay Profile',
            'label.endpoints': 'Apps Script Relays',
            'help.endpoints': 'Add one or more Apps Script relay URLs. Commas let Zyrln fail over automatically.',
            'label.authKey': 'Auth Key',
            'help.authKey': 'Secret key for your Apps Script relay. Use the same key on every Zyrln component.',
            'label.listen': 'HTTP',
            'help.listen': 'Local address browsers can use for HTTP and HTTPS traffic.',
            'label.socksListen': 'SOCKS5',
            'help.socksListen': 'Local address apps can use for SOCKS5 traffic.',
            'prompt.profileName': 'Profile name',
            'button.save': 'Save Settings',
            'button.addRelay': 'Add relay',
            'button.addProfile': 'Add Profile',
            'button.deleteProfile': 'Delete',
            'section.tools': 'Tools',
            'tool.diagnostics': 'Relay Check',
            'tool.diagnosticsDesc': 'Test the saved relay path',
            'button.runTest': 'Run',
            'tool.android': 'Mobile Export',
            'tool.androidDesc': 'Create Android import JSON',
            'button.export': 'Export',
            'tool.ping': 'Ping',
            'tool.pingDesc': 'Measure relay round-trip time',
            'button.ping': 'Ping',
            'tool.security': 'Certificate',
            'tool.securityDesc': 'Create the local CA file',
            'button.installCert': 'Install Certificate',
            'help.cert': 'Generate and download the CA certificate.',
            'tool.directMode': 'Direct (Google)',
            'tool.directModeDesc': 'Bypass relay for Google services',
            'button.directMode': 'Toggle direct mode',
            'stat.uptime': 'Uptime',
            'stat.requests': 'Requests',
            'placeholder.filter': 'Filter logs...',
            'button.clearLogs': 'Clear Logs',
            'button.exportLogs': 'Export Logs',
            'log.initial': 'System initialized. Ready for connection.',
            'modal.exportTitle': 'Export Configuration',
            'modal.exportDesc': 'Copy this JSON into your Android app:',
            'modal.mobileSync': 'Mobile Sync',
            'button.copyClose': 'Copy & Close',
            'progress.saving': 'Saving configuration...',
            'progress.starting': 'Starting HTTP and SOCKS5 listeners...',
            'progress.stopping': 'Stopping proxy...',
            'progress.diagnostics': 'Running connection diagnostics...',
            'progress.export': 'Generating export data...',
            'progress.cert': 'Generating certificate...',
            'toast.configSaved': 'Configuration saved',
            'toast.profileSaved': 'Profile saved',
            'toast.profileLoaded': 'Profile loaded',
            'toast.profileDeleted': 'Profile deleted',
            'toast.saveFailed': 'Save failed',
            'toast.saveError': 'Error saving configuration',
            'toast.configIncomplete': 'Saved configuration incomplete',
            'toast.actionFailed': 'Action failed',
            'toast.exportFailed': 'Export failed',
            'toast.certReady': 'Certificate Ready',
            'toast.copied': 'Copied to clipboard!',
            'log.configLoaded': 'Configuration loaded from config.env',
            'log.configUpdated': 'Configuration updated.',
            'log.profileLoaded': 'Profile loaded: {name}',
            'log.profileSaved': 'Profile saved: {name}',
            'log.profileDeleted': 'Profile deleted.',
            'log.cannotStart': 'Cannot start proxy: saved relay endpoint, auth key, and listen addresses are required.',
            'log.diagnosticsComplete': 'Diagnostics complete.',
            'log.diagnosticsFailed': 'Diagnostics failed: {error}',
            'log.exported': 'Exported mobile configuration to modal.',
            'log.openSave': 'Opening Save dialog...',
            'log.saveCancelled': 'Save cancelled.',
            'log.certRegenerated': 'Certificate regenerated. Writing to file...',
            'log.certSaved': 'Certificate saved successfully.',
            'log.certDownloaded': 'Certificate downloaded to your standard folder.'
        },
        fa: {
            'app.title': 'مرکز کنترل زیرلن',
            'brand.name': 'زیرلن',
            'button.connect': 'اتصال',
            'button.disconnect': 'قطع اتصال',
            'button.theme': 'تغییر پوسته',
            'button.close': 'بستن',
            'section.config': 'تنظیم اتصال',
            'label.endpoints': 'رله‌های Apps Script',
            'help.endpoints': 'یک یا چند آدرس رله Apps Script اضافه کنید. با کاما، زیرلن در صورت خطا خودکار سراغ بعدی می‌رود.',
            'label.authKey': 'کلید احراز هویت',
            'help.authKey': 'کلید محرمانه رله Apps Script. همین کلید باید در همه بخش‌های زیرلن استفاده شود.',
            'label.listen': 'HTTP',
            'help.listen': 'آدرس محلی که مرورگر برای ترافیک HTTP و HTTPS استفاده می‌کند.',
            'label.socksListen': 'SOCKS5',
            'help.socksListen': 'آدرس محلی که برنامه‌ها برای ترافیک SOCKS5 استفاده می‌کنند.',
            'prompt.profileName': 'نام پروفایل',
            'button.save': 'ذخیره تنظیمات',
            'button.addRelay': 'افزودن رله',
            'button.addProfile': 'افزودن پروفایل',
            'button.deleteProfile': 'حذف',
            'section.tools': 'ابزارها',
            'tool.diagnostics': 'بررسی رله',
            'tool.diagnosticsDesc': 'تست مسیر رله ذخیره‌شده',
            'button.runTest': 'اجرا',
            'tool.android': 'خروجی موبایل',
            'tool.androidDesc': 'ساخت JSON برای وارد کردن در اندروید',
            'button.export': 'خروجی',
            'tool.ping': 'پینگ',
            'tool.pingDesc': 'اندازه‌گیری زمان رفت‌وبرگشت رله',
            'button.ping': 'پینگ',
            'tool.security': 'گواهینامه',
            'tool.securityDesc': 'ساخت فایل CA محلی',
            'button.installCert': 'نصب گواهینامه',
            'help.cert': 'ساخت و دانلود گواهینامه CA',
            'tool.directMode': 'مستقیم (گوگل)',
            'tool.directModeDesc': 'اتصال مستقیم به سرویس‌های گوگل',
            'button.directMode': 'تغییر حالت مستقیم',
            'stat.uptime': 'زمان اجرا',
            'stat.requests': 'درخواست‌ها',
            'placeholder.filter': '...فیلتر لاگ‌ها',
            'button.clearLogs': 'پاک کردن لاگ‌ها',
            'button.exportLogs': 'دانلود لاگ',
            'modal.exportTitle': 'خروجی پیکربندی',
            'modal.exportDesc': 'این JSON را در اپ اندروید وارد کنید:',
            'modal.mobileSync': 'همگام‌سازی موبایل',
            'button.copyClose': 'کپی و بستن',
            'progress.saving': 'در حال ذخیره پیکربندی...',
            'progress.starting': 'در حال شروع شنونده‌های HTTP و SOCKS5...',
            'progress.stopping': 'در حال توقف پروکسی...',
            'progress.diagnostics': 'در حال اجرای عیب‌یابی اتصال...',
            'progress.export': 'در حال ساخت خروجی...',
            'progress.cert': 'در حال ساخت گواهینامه...',
            'toast.configSaved': 'پیکربندی ذخیره شد',
            'toast.profileSaved': 'پروفایل ذخیره شد',
            'toast.profileLoaded': 'پروفایل فعال شد',
            'toast.profileDeleted': 'پروفایل حذف شد',
            'toast.saveFailed': 'ذخیره ناموفق بود',
            'toast.saveError': 'خطا در ذخیره پیکربندی',
            'toast.configIncomplete': 'پیکربندی ذخیره‌شده کامل نیست',
            'toast.actionFailed': 'عملیات ناموفق بود',
            'toast.exportFailed': 'خروجی ناموفق بود',
            'toast.certReady': 'گواهینامه آماده است',
            'toast.copied': 'در کلیپ‌بورد کپی شد',
        }
    };

    let currentLang = localStorage.getItem('zyrln_lang') || 'en';
    const t = (key, params = {}) => {
        const value = translations[currentLang][key] || translations.en[key] || key;
        return Object.entries(params).reduce((text, [name, replacement]) => (
            text.replaceAll(`{${name}}`, replacement)
        ), value);
    };
    const tLog = (key, params = {}) => {
        const value = translations.en[key] || key;
        return Object.entries(params).reduce((text, [name, replacement]) => (
            text.replaceAll(`{${name}}`, replacement)
        ), value);
    };
    const legacyPersianLogs = new Map([
        ['پیکربندی از config.env خوانده شد', translations.en['log.configLoaded']],
        ['پیکربندی به‌روزرسانی شد.', translations.en['log.configUpdated']],
        ['در حال توقف پروکسی...', translations.en['progress.stopping']],
        ['در حال شروع پروکسی...', translations.en['progress.starting']],
        ['در حال شروع شنونده‌های HTTP و SOCKS5...', translations.en['progress.starting']],
        ['در حال اجرای عیب‌یابی اتصال...', translations.en['progress.diagnostics']],
        ['عیب‌یابی کامل شد.', translations.en['log.diagnosticsComplete']],
        ['پیکربندی موبایل در پنجره خروجی نمایش داده شد.', translations.en['log.exported']],
        ['در حال باز کردن پنجره ذخیره...', translations.en['log.openSave']],
        ['ذخیره لغو شد.', translations.en['log.saveCancelled']],
        ['گواهینامه دوباره ساخته شد. در حال نوشتن فایل...', translations.en['log.certRegenerated']],
        ['گواهینامه با موفقیت ذخیره شد.', translations.en['log.certSaved']],
        ['گواهینامه در پوشه دانلود پیش‌فرض ذخیره شد.', translations.en['log.certDownloaded']],
    ]);
    const normalizeLogMessage = (msg) => legacyPersianLogs.get(msg) || msg;

    // State management
    let proxyLogSeq = 0;

    window.__ZYRLN_STATE__ = {
        running: false,
        uptime: '00:00:00',
        logs: [],
        probesRunning: false,
        lastSavedConfig: {},
        profiles: [],
        activating: false,
        version: '',
        os: '',
        arch: ''
    };

    // DOM Elements
    const configForm = document.getElementById('configForm');
    const toggleProxyBtn = document.getElementById('toggleProxyBtn');
    const logOutput = document.getElementById('logOutput');
    const clearLogsBtn = document.getElementById('clearLogsBtn');
    const exportLogsBtn = document.getElementById('exportLogsBtn');
    const btnRegenCA = document.getElementById('btnRegenCA');
    const runProbesBtn = document.getElementById('runProbesBtn');
    const exportMobileBtn = document.getElementById('exportMobileBtn');
    const pingBtn = document.getElementById('pingBtn');
    const directModeToggle = document.getElementById('directModeToggle');
    const pingResult = document.getElementById('pingResult');
    const logFilter = document.getElementById('logFilter');
    const toggleAuthVisible = document.getElementById('toggleAuthVisible');
    const authKeyInput = document.getElementById('auth-key');
    const languageToggleBtn = document.getElementById('languageToggleBtn');
    const themeToggleBtn = document.getElementById('themeToggleBtn');
    const addProfileBtn = document.getElementById('addProfileBtn');
    const deleteProfileBtn = document.getElementById('deleteProfileBtn');
    const profilePanel = document.querySelector('.profile-actions');
    const configSection = document.querySelector('.config-section');

    // Custom select state
    const profileSelectWrapper = document.getElementById('profileSelectWrapper');
    const profileSelectTrigger = document.getElementById('profileSelectTrigger');
    const profileSelectLabel = document.getElementById('profileSelectLabel');
    const profileSelectDropdown = document.getElementById('profileSelectDropdown');
    let profileSelectValue = '';

    // URL Rows — replaces the textarea for fronted-appscript-url
    const urlRowsContainer = document.getElementById('urlRows');
    const urlHiddenInput = document.getElementById('fronted-appscript-url');
    const addUrlRowBtn = document.getElementById('addUrlRowBtn');

    function syncUrlHidden() {
        const vals = [...urlRowsContainer.querySelectorAll('.url-row-input')]
            .map(i => i.value.trim()).filter(Boolean);
        urlHiddenInput.value = vals.join(',');
        validateInputs();
    }

    function addUrlRow(value = '') {
        const row = document.createElement('div');
        row.className = 'url-row';
        const input = document.createElement('input');
        input.type = 'text';
        input.className = 'url-row-input';
        input.placeholder = 'https://script.google.com/macros/s/.../exec';
        input.value = value;
        input.autocomplete = 'off';
        input.spellcheck = false;
        input.addEventListener('input', syncUrlHidden);
        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'url-row-remove';
        removeBtn.innerHTML = '<svg class="ui-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6 6 18"></path><path d="m6 6 12 12"></path></svg>';
        removeBtn.addEventListener('click', () => {
            row.remove();
            syncUrlHidden();
            // Always keep at least one row
            if (urlRowsContainer.querySelectorAll('.url-row').length === 0) addUrlRow();
        });
        row.appendChild(input);
        row.appendChild(removeBtn);
        urlRowsContainer.appendChild(row);
        syncUrlHidden();
        return input;
    }

    function setUrlRows(commaValue) {
        urlRowsContainer.innerHTML = '';
        const urls = (commaValue || '').split(',').map(s => s.trim()).filter(Boolean);
        if (urls.length === 0) {
            addUrlRow();
        } else {
            urls.forEach(u => addUrlRow(u));
        }
    }

    addUrlRowBtn.addEventListener('click', () => {
        const input = addUrlRow();
        input.focus();
    });

    // Mimic native select API used throughout the code
    const profileSelect = {
        get value() { return profileSelectValue; },
        set value(v) {
            profileSelectValue = v;
            const opt = [...profileSelectDropdown.querySelectorAll('li')].find(li => li.dataset.value === v);
            profileSelectLabel.textContent = opt ? opt.textContent : '—';
            profileSelectDropdown.querySelectorAll('li').forEach(li =>
                li.classList.toggle('selected', li.dataset.value === v));
        },
        get disabled() { return profileSelectWrapper.classList.contains('disabled'); },
        set disabled(v) { profileSelectWrapper.classList.toggle('disabled', v); },
        get options() { return profileSelectDropdown.querySelectorAll('li'); },
        addEventListener(evt, fn) {
            if (evt === 'change') profileSelectWrapper._changeHandlers = (profileSelectWrapper._changeHandlers || []).concat(fn);
        }
    };

    // Initialize URL rows after profileSelect is defined
    setUrlRows('');

    profileSelectTrigger.addEventListener('click', () => {
        if (profileSelectWrapper.classList.contains('disabled')) return;
        profileSelectWrapper.classList.toggle('open');
        profileSelectTrigger.setAttribute('aria-expanded', profileSelectWrapper.classList.contains('open'));
    });

    profileSelectDropdown.addEventListener('click', (e) => {
        const li = e.target.closest('li');
        if (!li) return;
        profileSelect.value = li.dataset.value;
        profileSelectWrapper.classList.remove('open');
        profileSelectTrigger.setAttribute('aria-expanded', 'false');
        (profileSelectWrapper._changeHandlers || []).forEach(fn => fn());
    });

    document.addEventListener('click', (e) => {
        if (!profileSelectWrapper.contains(e.target)) {
            profileSelectWrapper.classList.remove('open');
            profileSelectTrigger.setAttribute('aria-expanded', 'false');
        }
    });

    // UI Feedback Elements
    let _loadingToast = null;

    // Modal Elements
    const modalOverlay = document.getElementById('modalOverlay');
    const modalTextArea = document.getElementById('modalTextArea');
    const modalActionBtn = document.getElementById('modalActionBtn');
    const closeModal = document.getElementById('closeModal');
    const iconSVG = {
        sun: '<svg class="ui-icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4"></circle><path d="M12 2v2"></path><path d="M12 20v2"></path><path d="m4.9 4.9 1.4 1.4"></path><path d="m17.7 17.7 1.4 1.4"></path><path d="M2 12h2"></path><path d="M20 12h2"></path><path d="m6.3 17.7-1.4 1.4"></path><path d="m19.1 4.9-1.4 1.4"></path></svg>',
        moon: '<svg class="ui-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M20.9 13.5A8.5 8.5 0 0 1 10.5 3.1 7 7 0 1 0 20.9 13.5Z"></path></svg>'
    };

    // Config Management
    async function loadConfig(silent = false) {
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            window.__ZYRLN_STATE__.lastSavedConfig = config;
            for (const [key, value] of Object.entries(config)) {
                if (key === 'fronted-appscript-url') {
                    setUrlRows(value);
                } else {
                    const input = document.getElementById(key);
                    if (input) input.value = value;
                }
            }
            if (!silent) showLog(tLog('log.configLoaded'));
            validateInputs();
            await loadProfiles();
        } catch (err) {
            showLog(`Error loading config: ${err.message}`, 'error');
        }
    }

    async function loadProfiles() {
        try {
            const response = await fetch('/api/profiles');
            const profiles = await response.json();
            window.__ZYRLN_STATE__.profiles = Array.isArray(profiles) ? profiles : [];
            renderProfiles();
        } catch (err) {
            showLog(`Error loading profiles: ${err.message}`, 'error');
        }
    }

    function currentFormConfig() {
        const formData = new FormData(configForm);
        const cfg = Object.fromEntries(formData.entries());
        // listen inputs live outside the form (in the header)
        const listenEl = document.getElementById('listen');
        const socksEl = document.getElementById('socks-listen');
        if (listenEl) cfg['listen'] = listenEl.value;
        if (socksEl) cfg['socks-listen'] = socksEl.value;
        return cfg;
    }

    function currentProfileConfig() {
        const config = currentFormConfig();
        return {
            'fronted-appscript-url': config['fronted-appscript-url'] || '',
            'auth-key': config['auth-key'] || ''
        };
    }

    function renderProfiles() {
        const selected = profileSelectValue;
        profileSelectDropdown.innerHTML = '';

        // Always show direct-only entry at the top
        const directLi = document.createElement('li');
        directLi.dataset.value = '__direct__';
        directLi.textContent = '⚡ ' + t('tool.directMode');
        profileSelectDropdown.appendChild(directLi);

        window.__ZYRLN_STATE__.profiles.forEach(profile => {
            const li = document.createElement('li');
            li.dataset.value = profile.id;
            li.textContent = profile.name || 'Profile';
            profileSelectDropdown.appendChild(li);
        });

        const items = [...profileSelectDropdown.querySelectorAll('li')];
        const stillExists = items.some(li => li.dataset.value === selected);
        if (stillExists) {
            profileSelect.value = selected;
        } else if (items.length > 0) {
            profileSelect.value = items[0].dataset.value;
        } else {
            profileSelect.value = '';
        }
        validateInputs();
    }

    configForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showProgress(t('progress.saving'));
        const data = currentFormConfig();

        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (response.ok) {
                window.__ZYRLN_STATE__.lastSavedConfig = data;
                // If a relay profile is selected, sync its stored config so switching away/back doesn't revert edits
                if (profileSelect.value && profileSelect.value !== '__direct__') {
                    const currentProfile = selectedProfile();
                    const pConfig = { 'fronted-appscript-url': data['fronted-appscript-url'] || '', 'auth-key': data['auth-key'] || '' };
                    await fetch('/api/profiles', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ id: profileSelect.value, name: currentProfile?.name || '', config: pConfig })
                    });
                    await loadProfiles();
                }
                showToast(t('toast.configSaved'));
                showLog(tLog('log.configUpdated'));
                playMotion(configForm, 'motion-confirm');
                validateInputs();
            } else {
                const errText = await response.text();
                showLog(`Save failed: ${errText}`, 'error');
                showToast(t('toast.saveFailed'), 'error');
            }
        } catch (err) {
            showToast(t('toast.saveError'), 'error');
            showLog(`Save failed: ${err.message}`, 'error');
        } finally {
            hideProgress();
        }
    });

    const profileNameOverlay = document.getElementById('profileNameOverlay');
    const profileNameInput = document.getElementById('profileNameInput');
    const profileNameConfirm = document.getElementById('profileNameConfirm');
    const closeProfileNameModal = document.getElementById('closeProfileNameModal');

    function promptProfileName(suggested) {
        return new Promise((resolve) => {
            profileNameInput.value = suggested;
            profileNameOverlay.classList.add('active');
            profileNameInput.focus();
            profileNameInput.select();

            const finish = (value) => {
                profileNameOverlay.classList.remove('active');
                profileNameConfirm.removeEventListener('click', onConfirm);
                closeProfileNameModal.removeEventListener('click', onCancel);
                profileNameInput.removeEventListener('keydown', onKey);
                resolve(value);
            };
            const onConfirm = () => finish(profileNameInput.value.trim() || suggested);
            const onCancel = () => finish(null);
            const onKey = (e) => {
                if (e.key === 'Enter') finish(profileNameInput.value.trim() || suggested);
                if (e.key === 'Escape') finish(null);
            };
            profileNameConfirm.addEventListener('click', onConfirm);
            closeProfileNameModal.addEventListener('click', onCancel);
            profileNameInput.addEventListener('keydown', onKey);
        });
    }

    async function saveProfile() {
        if (window.__ZYRLN_STATE__.running) return;
        const config = currentProfileConfig();
        const suggested = profileNameFromConfig(config);
        const name = await promptProfileName(suggested);
        if (name === null) return;

        showProgress(t('progress.saving'));
        try {
            const response = await fetch('/api/profiles', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: '',
                    name,
                    config
                })
            });
            if (!response.ok) throw new Error(await response.text());
            const profile = await response.json();
            showToast(t('toast.profileSaved'));
            showLog(tLog('log.profileSaved', { name: profile.name || 'Profile' }));
            await loadProfiles();
            profileSelect.value = profile.id;
            playMotion(profilePanel, 'motion-confirm');
            playMotion(profileSelectWrapper, 'motion-select');
        } catch (err) {
            showToast(`${t('toast.saveFailed')}: ${err.message}`, 'error');
            showLog(`Profile save failed: ${err.message}`, 'error');
        } finally {
            hideProgress();
        }
    }

    addProfileBtn.addEventListener('click', async () => {
        await saveProfile();
    });

    async function activateSelectedProfile(silent = false) {
        if (window.__ZYRLN_STATE__.running || !profileSelect.value) return;
        const profile = selectedProfile();
        window.__ZYRLN_STATE__.activating = true;
        validateInputs();
        if (!silent) showProgress(t('progress.saving'));
        try {
            const response = await fetch('/api/profiles/activate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: profileSelect.value })
            });
            if (!response.ok) throw new Error(await response.text());
            await loadConfig(silent);
            profileSelect.value = profile?.id || '';
            if (!silent) {
                showToast(t('toast.profileLoaded'));
                showLog(tLog('log.profileLoaded', { name: profile?.name || 'Profile' }));
                playMotion(configSection, 'motion-apply');
                playMotion(profileSelectWrapper, 'motion-select');
            }
        } catch (err) {
            showToast(`${t('toast.actionFailed')}: ${err.message}`, 'error');
            showLog(`Profile load failed: ${err.message}`, 'error');
        } finally {
            window.__ZYRLN_STATE__.activating = false;
            if (!silent) hideProgress();
            validateInputs();
        }
    }

    deleteProfileBtn.addEventListener('click', async () => {
        if (window.__ZYRLN_STATE__.running || !profileSelect.value) return;
        showProgress(t('progress.saving'));
        try {
            const response = await fetch(`/api/profiles?id=${encodeURIComponent(profileSelect.value)}`, {
                method: 'DELETE'
            });
            if (!response.ok) throw new Error(await response.text());
            profileSelect.value = '';
            await loadProfiles();
            if (window.__ZYRLN_STATE__.profiles.length === 0) {
                // No profiles left — clear form fields
                setUrlRows('');
                const keyEl = document.getElementById('auth-key');
                if (keyEl) keyEl.value = '';
                window.__ZYRLN_STATE__.lastSavedConfig = {};
            } else {
                // Activate next profile silently so config.env stays in sync
                await activateSelectedProfile(true);
            }
            showToast(t('toast.profileDeleted'));
            showLog(tLog('log.profileDeleted'));
            playMotion(profilePanel, 'motion-delete');
        } catch (err) {
            showToast(`${t('toast.actionFailed')}: ${err.message}`, 'error');
            showLog(`Profile delete failed: ${err.message}`, 'error');
        } finally {
            hideProgress();
        }
    });

    function selectedProfile() {
        return window.__ZYRLN_STATE__.profiles.find(profile => profile.id === profileSelect.value);
    }

    function profileNameFromConfig(config) {
        const raw = (config['fronted-appscript-url'] || '').split(',')[0].trim();
        try {
            return new URL(raw).hostname || 'Profile';
        } catch (_) {
            return 'Profile';
        }
    }

    // Proxy Control
    toggleProxyBtn.addEventListener('click', async () => {
        if (window.__ZYRLN_STATE__.activating) return;
        const isRunning = window.__ZYRLN_STATE__.running;
        if (!isRunning && !hasSavedConfig()) {
            showLog(tLog('log.cannotStart'), 'error');
            showToast(t('toast.configIncomplete'), 'error');
            validateInputs();
            return;
        }
        if (!isRunning && profileSelect.value === '__direct__') {
            // Direct-only mode: enable direct, skip relay config validation
            await fetch('/api/settings', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ directEnabled: true }),
            });
            directModeToggle.classList.add('active');
        } else if (!isRunning) {
            const savedUrl = (window.__ZYRLN_STATE__.lastSavedConfig?.['fronted-appscript-url'] || '').split(',')[0].trim();
            if (!savedUrl.startsWith('https://script.google.com/')) {
                showLog('Relay URL is not a valid Apps Script URL — cannot connect.', 'error');
                showToast(t('toast.configIncomplete'), 'error');
                return;
            }
        }
        const endpoint = isRunning ? '/api/stop' : '/api/start';

        showProgress(isRunning ? t('progress.stopping') : t('progress.starting'));
        try {
            const response = await fetch(endpoint, { method: 'POST' });
            if (response.ok) {
                playMotion(toggleProxyBtn, isRunning ? 'motion-soft' : 'motion-confirm');
                updateStatus();
                validateInputs();
            } else {
                const errText = await response.text();
                showLog(`Failed: ${errText}`, 'error');
                showToast(`${t('toast.actionFailed')}: ${errText}`, 'error');
            }
        } catch (err) {
            showLog(`Connection error: ${err.message}`, 'error');
        } finally {
            setTimeout(hideProgress, 500);
        }
    });

    // Tools & Actions
    runProbesBtn.addEventListener('click', async () => {
        if (window.__ZYRLN_STATE__.probesRunning) return;

        const savedUrl = (window.__ZYRLN_STATE__.lastSavedConfig?.['fronted-appscript-url'] || '').trim();
        const firstUrl = savedUrl.split(',')[0].trim();
        if (!firstUrl.startsWith('https://script.google.com/')) {
            showLog('Relay URL is not a valid Apps Script URL — diagnostics aborted.', 'error');
            showToast(t('toast.actionFailed'), 'error');
            return;
        }

        window.__ZYRLN_STATE__.probesRunning = true;
        setButtonDisabled(runProbesBtn, true);
        showProgress(t('progress.diagnostics'));
        showLog(tLog('progress.diagnostics'));

        try {
            const response = await fetch('/api/probes');
            if (!response.ok) throw new Error(await response.text());
            const results = await response.json();
            if (!Array.isArray(results)) throw new Error('Unexpected response');
            results.forEach(r => {
                const name = r.probe?.name || r.id || 'probe';
                const status = r.ok ? 'OK' : 'FAIL';
                showLog(`${name}: ${status} (${r.duration_ms}ms)`, r.ok ? 'info' : 'error');
            });
            showLog(tLog('log.diagnosticsComplete'));
        } catch (err) {
            showLog(tLog('log.diagnosticsFailed', { error: err.message }), 'error');
        } finally {
            window.__ZYRLN_STATE__.probesRunning = false;
            hideProgress();
            validateInputs();
        }
    });

    exportMobileBtn.addEventListener('click', async () => {
        showProgress(t('progress.export'));
        try {
            const response = await fetch('/api/export');
            if (!response.ok) throw new Error(await response.text());
            const config = await response.json();
            const jsonStr = JSON.stringify(config, null, 2);
            openModal(t('modal.mobileSync'), jsonStr);
            showLog(tLog('log.exported'));
            playMotion(exportMobileBtn, 'motion-confirm');
        } catch (err) {
            showToast(`${t('toast.exportFailed')}: ${err.message}`, 'error');
            showLog(`Export failed: ${err.message}`, 'error');
        } finally {
            hideProgress();
        }
    });

    pingBtn.addEventListener('click', async () => {

        const PING_QUALITY = [
            { max: 199, class: 'text-success', name: 'GOOD' },
            { max: 949, class: 'text-warning', name: 'MEDIUM' },
            { max: Infinity, class: 'text-danger', name: 'BAD' }
        ];

        const getPingQuality = (ms) => PING_QUALITY.find(range => ms <= range.max);

        const updatePingUI = (ms = null) => {
            const className = getPingQuality(ms).class
            pingResult.classList.add(className);
        }

        pingResult.textContent = '…';
        setButtonDisabled(pingBtn, true);
        pingResult.classList.remove('text-success', 'text-warning', 'text-danger');
        try {
            const res = await fetch('/api/ping', { method: 'POST' });
            const data = await res.json();
            if (data.ok) {
                pingResult.textContent = `${data.ms} ms`;
                showLog(`Ping: ${data.ms} ms`, 'info');
                playMotion(pingBtn, 'motion-confirm');
                updatePingUI(data.ms)
            } else {
                pingResult.textContent = 'failed';
                showLog(`Ping failed: ${data.error}`, 'error');
                updatePingUI()
            }
        } catch (err) {
            pingResult.textContent = 'failed';
            showLog(`Ping error: ${err.message}`, 'error');
            updatePingUI()
        } finally {
            setButtonDisabled(pingBtn, false);
        }
    });

    btnRegenCA.addEventListener('click', async () => {
        showLog(tLog('log.openSave'), 'info');

        try {
            let handle = null;
            // 1. Try to get a save handle immediately (User Activation)
            if ('showSaveFilePicker' in window) {
                try {
                    handle = await window.showSaveFilePicker({
                        suggestedName: 'zyrln-ca.pem',
                        types: [{
                            description: 'PEM Certificate',
                            accept: { 'application/x-x509-ca-cert': ['.pem'] },
                        }],
                    });
                } catch (err) {
                    if (err.name === 'AbortError') {
                        showLog(tLog('log.saveCancelled'), 'info');
                        return;
                    }
                    console.error('Picker failed:', err);
                }
            }

            showProgress(t('progress.cert'));
            // 2. Regenerate on backend
            const response = await fetch('/api/init-ca', { method: 'POST' });
            if (!response.ok) throw new Error('Generation failed');

            showLog(`${tLog('log.certRegenerated')}`, 'info');

            // 3. Download the data
            const certRes = await fetch('/api/download-ca');
            const blob = await certRes.blob();

            // 4. Save to handle OR fallback
            if (handle) {
                const writable = await handle.createWritable();
                await writable.write(blob);
                await writable.close();
                showLog(`${tLog('log.certSaved')}`, 'info');
            } else {
                // Fallback for non-supported browsers
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'zyrln-ca.pem';
                a.click();
                window.URL.revokeObjectURL(url);
                showLog(`${tLog('log.certDownloaded')}`, 'info');
            }

            showToast(t('toast.certReady'));
            playMotion(btnRegenCA, 'motion-confirm');
            updateStatus();
        } catch (err) {
            console.error('Action Error:', err);
            showLog(`Error: ${err.message}`, 'error');
            showToast(t('toast.actionFailed'));
        } finally {
            hideProgress();
        }
    });

    // Direct mode toggle
    async function loadDirectMode() {
        try {
            const res = await fetch('/api/settings');
            if (!res.ok) return;
            const data = await res.json();
            directModeToggle.classList.toggle('active', !!data.directEnabled);
            directModeToggle.title = data.directEnabled ? 'Direct mode ON — Google bypasses relay' : 'Direct mode OFF';
        } catch (_) { }
    }

    directModeToggle.addEventListener('click', async () => {
        try {
            const isOn = directModeToggle.classList.contains('active');
            const next = !isOn;
            await fetch('/api/settings', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ directEnabled: next }),
            });
            directModeToggle.classList.toggle('active', next);
            directModeToggle.title = next ? 'Direct mode ON — Google bypasses relay' : 'Direct mode OFF';
            showLog(next ? 'Direct mode enabled — Google services bypass relay' : 'Direct mode disabled — all traffic via relay', 'info');
        } catch (err) {
            showLog(`Error: ${err.message}`, 'error');
        }
    });

    loadDirectMode();

    // Log Filtering
    logFilter.addEventListener('input', (e) => {
        const term = e.target.value.toLowerCase();
        const entries = logOutput.querySelectorAll('.log-entry');
        entries.forEach(entry => {
            const text = entry.textContent.toLowerCase();
            entry.style.display = text.includes(term) ? 'block' : 'none';
        });
    });

    // Modal Logic
    function openModal(title, content) {
        document.getElementById('modalTitle').textContent = title;
        modalTextArea.value = content;
        modalOverlay.classList.add('active');
    }

    closeModal.onclick = () => modalOverlay.classList.remove('active');
    window.addEventListener('click', (e) => {
        if (e.target === modalOverlay) modalOverlay.classList.remove('active');
        if (e.target === profileNameOverlay) {
            profileNameOverlay.classList.remove('active');
        }
    });

    modalActionBtn.onclick = async () => {
        await navigator.clipboard.writeText(modalTextArea.value);
        showToast(t('toast.copied'));
        modalOverlay.classList.remove('active');
    };

    // UI Helpers
    function showProgress(text) {
        if (_loadingToast) _loadingToast.remove();
        const container = document.getElementById('toastContainer');
        _loadingToast = document.createElement('div');
        _loadingToast.className = 'toast loading';
        _loadingToast.innerHTML = `<div class= "toast-spinner"></div> <span>${text}</span>`;
        container.appendChild(_loadingToast);
    }

    function hideProgress() {
        if (_loadingToast) {
            _loadingToast.classList.add('leaving');
            setTimeout(() => { if (_loadingToast) { _loadingToast.remove(); _loadingToast = null; } }, 300);
        }
    }

    const eyeIcon = `<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
            <circle cx="12" cy="12" r="3"></circle>
        </svg > `;
    const eyeOffIcon = `<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" >
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
            <line x1="1" y1="1" x2="23" y2="23"></line>
        </svg> `;

    toggleAuthVisible.innerHTML = eyeIcon;
    toggleAuthVisible.onclick = () => {
        const isPassword = authKeyInput.type === 'password';
        authKeyInput.type = isPassword ? 'text' : 'password';
        toggleAuthVisible.innerHTML = isPassword ? eyeOffIcon : eyeIcon;
    };

    function showLog(msg, type = 'info', savedTime = null) {
        msg = normalizeLogMessage(msg);
        const entry = document.createElement('div');
        entry.className = `log-entry ${type}`;
        const time = savedTime || new Date().toLocaleTimeString([], { hour12: false });
        entry.textContent = `[${time}]${msg}`;
        entry.dataset.time = time;
        entry.dataset.msg = msg;
        entry.dataset.type = type;

        // Save to state and local storage
        window.__ZYRLN_STATE__.logs.push({ time, msg, type });
        if (window.__ZYRLN_STATE__.logs.length > 500) window.__ZYRLN_STATE__.logs.shift();
        localStorage.setItem('zyrln_logs', JSON.stringify(window.__ZYRLN_STATE__.logs));

        // Apply current filter
        const term = logFilter.value.toLowerCase();
        if (term && !entry.textContent.toLowerCase().includes(term)) {
            entry.style.display = 'none';
        }

        logOutput.appendChild(entry);
        logOutput.scrollTop = logOutput.scrollHeight;
    }

    function playMotion(el, className) {
        if (!el) return;
        el.classList.remove(className);
        void el.offsetWidth;
        el.classList.add(className);
    }

    function showToast(msg, type = 'info') {
        const container = document.getElementById('toastContainer');
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.textContent = msg;
        container.appendChild(toast);
        setTimeout(() => {
            toast.classList.add('leaving');
            setTimeout(() => toast.remove(), 300);
        }, 3000);
    }

    function applyLocalization() {
        document.documentElement.lang = currentLang;
        document.documentElement.dir = currentLang === 'fa' ? 'rtl' : 'ltr';
        document.title = t('app.title');
        document.body.classList.toggle('rtl', currentLang === 'fa');
        languageToggleBtn.textContent = currentLang === 'fa' ? 'EN' : 'FA';

        document.querySelectorAll('[data-i18n]').forEach(el => {
            el.textContent = t(el.dataset.i18n);
        });
        document.querySelectorAll('[data-i18n-title]').forEach(el => {
            const text = t(el.dataset.i18nTitle);
            el.title = text;
            el.setAttribute('aria-label', text);
        });
        document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
            el.placeholder = t(el.dataset.i18nPlaceholder);
        });
        renderProfiles();
        updateStatusText();
    }

    function updateStatusText() {
        if (window.__ZYRLN_STATE__.running) {
            toggleProxyBtn.title = t('button.disconnect');
            toggleProxyBtn.setAttribute('aria-label', t('button.disconnect'));
        } else {
            toggleProxyBtn.title = t('button.connect');
            toggleProxyBtn.setAttribute('aria-label', t('button.connect'));
        }
    }

    async function updateStatus() {
        try {
            const response = await fetch('/api/status');
            if (!response.ok) throw new Error(await response.text());
            const status = await response.json();
            const wasRunning = window.__ZYRLN_STATE__.running;
            window.__ZYRLN_STATE__.running = status.running;
            if (!status.running && wasRunning) proxyLogSeq = 0;

            if (status.running) {
                toggleProxyBtn.classList.add('danger');
            } else {
                toggleProxyBtn.classList.remove('danger');
            }
            updateStatusText();

            document.getElementById('uptimeValue').textContent = status.uptime || '00:00:00';
            document.getElementById('requestCount').textContent = status.requests || '0';
            if (status.version) window.__ZYRLN_STATE__.version = status.version;
            if (status.os) window.__ZYRLN_STATE__.os = status.os;
            if (status.arch) window.__ZYRLN_STATE__.arch = status.arch;

            // Ensure inputs are disabled/enabled based on running state
            validateInputs();
        } catch (err) {
            setButtonDisabled(toggleProxyBtn, true);
        }
    }

    languageToggleBtn.onclick = () => {
        currentLang = currentLang === 'fa' ? 'en' : 'fa';
        localStorage.setItem('zyrln_lang', currentLang);
        applyLocalization();
    };

    function applyTheme() {
        const theme = localStorage.getItem('zyrln_theme') || 'dark';
        document.body.classList.toggle('light-mode', theme === 'light');
        document.body.classList.toggle('dark-mode', theme !== 'light');
        themeToggleBtn.innerHTML = theme === 'light' ? iconSVG.moon : iconSVG.sun;
    }

    themeToggleBtn.onclick = () => {
        const currentTheme = document.body.classList.contains('light-mode') ? 'light' : 'dark';
        localStorage.setItem('zyrln_theme', currentTheme === 'light' ? 'dark' : 'light');
        applyTheme();
    };

    clearLogsBtn.onclick = () => {
        logOutput.innerHTML = '';
        window.__ZYRLN_STATE__.logs = [];
        localStorage.removeItem('zyrln_logs');
    };

    exportLogsBtn.onclick = () => {
        const lines = window.__ZYRLN_STATE__.logs.map(l => `[${l.time}]${l.msg}`).join('\n');
        if (!lines) return;
        const s = window.__ZYRLN_STATE__;
        const now = new Date().toISOString().replace('T', ' ').slice(0, 19) + ' UTC';
        const header = [
            `Zyrln Desktop v${s.version || 'unknown'}`,
            `OS: ${s.os || 'unknown'} / ${s.arch || 'unknown'}`,
            `Time: ${now}`,
            '---',
            ''
        ].join('\n');
        const blob = new Blob([header + lines], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `zyrln - log - ${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.txt`;
        a.click();
        URL.revokeObjectURL(url);
    };

    // Initialize
    const savedLogs = localStorage.getItem('zyrln_logs');
    if (savedLogs) {
        try {
            const logs = JSON.parse(savedLogs);
            logs.forEach(l => showLog(normalizeLogMessage(l.msg), l.type, l.time));
        } catch (e) { localStorage.removeItem('zyrln_logs'); }
    }

    function validateInputs() {
        const isRunning = window.__ZYRLN_STATE__.running;

        // Disable configuration section ONLY if running
        const configInputs = configForm.querySelectorAll('input, textarea, button[type="submit"]');
        configInputs.forEach(el => {
            el.disabled = isRunning;
            if (el.classList.contains('save-btn')) {
                el.style.opacity = isRunning ? '0.5' : '1';
            }
        });
        urlRowsContainer.querySelectorAll('.url-row-input, .url-row-remove').forEach(el => { el.disabled = isRunning; });
        if (addUrlRowBtn) { addUrlRowBtn.disabled = isRunning; addUrlRowBtn.style.opacity = isRunning ? '0.5' : '1'; }

        // Header listen inputs live outside the form
        document.querySelectorAll('.listen-input').forEach(el => {
            el.disabled = isRunning;
        });

        // Basic requirement: fields must not be empty
        const hasRunnableConfig = hasSavedConfig();
        const hasRelay = hasRelayConfig();
        const isActivating = window.__ZYRLN_STATE__.activating;

        setButtonDisabled(toggleProxyBtn, isActivating || (!isRunning && !hasRunnableConfig));

        // These actions require a relay config (Apps Script URL + key), not just direct mode.
        if (runProbesBtn) setButtonDisabled(runProbesBtn, !hasRelay || window.__ZYRLN_STATE__.probesRunning);
        if (exportMobileBtn) setButtonDisabled(exportMobileBtn, !hasRelay || isActivating);

        // Install Certificate is special — it only needs the server to be ready, but we'll keep it enabled
        if (btnRegenCA) {
            setButtonDisabled(btnRegenCA, false);
        }
        profileSelect.disabled = isRunning;
        [addProfileBtn, deleteProfileBtn].forEach(el => {
            if (!el) return;
            const needsSelectedProfile = el === deleteProfileBtn;
            setButtonDisabled(el, isRunning || (needsSelectedProfile && (!profileSelect.value || profileSelect.value === '__direct__')));
        });
    }

    function hasSavedConfig() {
        if (profileSelect.value === '__direct__') return true;
        return hasRelayConfig();
    }

    function hasRelayConfig() {
        const config = window.__ZYRLN_STATE__.lastSavedConfig || {};
        const url = (config['fronted-appscript-url'] || '').trim();
        const key = (config['auth-key'] || '').trim();
        return url !== '' && key !== '';
    }

    function setButtonDisabled(btn, disabled) {
        btn.disabled = disabled;
        btn.setAttribute('aria-disabled', disabled ? 'true' : 'false');
        btn.style.opacity = disabled ? '0.5' : '1';
    }

    // Listen for input changes
    ['fronted-appscript-url', 'auth-key', 'listen', 'socks-listen'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.addEventListener('input', validateInputs);
    });
    profileSelect.addEventListener('change', () => {
        if (window.__ZYRLN_STATE__.activating) return;
        validateInputs();
        playMotion(profileSelectWrapper, 'motion-select');
        if (profileSelect.value === '__direct__') {
            setUrlRows('');
            document.getElementById('auth-key').value = '';
        } else {
            activateSelectedProfile();
        }
    });

    applyTheme();
    applyLocalization();
    showLog(t('log.initial'), 'system');
    loadConfig().then(() => {
        validateInputs();
        // Activate the auto-selected profile silently so config.env matches what's shown
        if (profileSelect.value && profileSelect.value !== '__direct__') activateSelectedProfile(true);
    });
    async function pollProxyLogs() {
        if (!window.__ZYRLN_STATE__.running) return;
        try {
            const res = await fetch(`/ api / logs ? seq = ${proxyLogSeq}`);
            if (!res.ok) return;
            const newSeq = parseInt(res.headers.get('X-Log-Seq') || '0', 10);
            // Detect server restart: seq reset below our cursor — resync
            if (newSeq < proxyLogSeq) proxyLogSeq = 0;
            const text = await res.text();
            if (text.trim()) {
                text.trim().split('\n').forEach(line => {
                    const tab = line.indexOf('\t');
                    if (tab < 0) return;
                    const level = line.substring(0, tab);
                    const msg = line.substring(tab + 1);
                    const type = level === 'error' ? 'error' : level === 'system' ? 'system' : 'info';
                    showLog(msg, type);
                });
            }
            proxyLogSeq = newSeq;
        } catch (_) { }
    }

    setInterval(updateStatus, 2000);
    setInterval(pollProxyLogs, 500);
    updateStatus();
});
