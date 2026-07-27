;(function (window, $) {
    if (!$) {
        return;
    }

    var nativeSubmit = window.HTMLFormElement && window.HTMLFormElement.prototype.submit;
    var SINGLE_PANEL_ID = 'app-panel-0';
    var menuTree = window.ADMIN_MENU_TREE || [];
    var loadedScripts = {};
    var $taskContentInner;
    var $loading;
    var elementApi;
    var tabsApi;
    var workspaceBooted = false;
    var homeApp = {
        url: '',
        id: '0',
        name: '首页'
    };

    function escapeHtml(value) {
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    function toUrl(url) {
        return new URL(url || '', window.location.origin);
    }

    function toAppUrl(url) {
        var parsed = toUrl(url);
        return parsed.pathname + parsed.search + parsed.hash;
    }

    function isHomeUrl(url) {
        if (!url || !homeApp.url) {
            return false;
        }
        try {
            return toAppUrl(url) === toAppUrl(homeApp.url);
        } catch (e) {
            return false;
        }
    }

    function isSameOrigin(url) {
        try {
            return toUrl(url).origin === window.location.origin;
        } catch (e) {
            return false;
        }
    }

    function isWorkspacePage(url) {
        if (!url || /^javascript:/i.test(url) || /^#/i.test(url) || !isSameOrigin(url)) {
            return false;
        }
        var path = toUrl(url).pathname;
        return path.indexOf('/admin/') === 0 ||
            path.indexOf('/user/') === 0 ||
            path.indexOf('/portal/') === 0;
    }

    function isExportUrl(url) {
        var parsed = toUrl(url);
        return /\/export(\.html)?(?:$|\?)/i.test(parsed.pathname + parsed.search);
    }

    function requestHeaders() {
        var headers = {'X-Requested-With': 'XMLHttpRequest'};
        var token = (window.GV && window.GV.ADMIN_TOKEN) || window.localStorage.getItem('token') || '';
        if (token) {
            window.localStorage.setItem('token', token);
            headers['XX-Token'] = token;
            headers.Authorization = token;
        }
        return headers;
    }

    function showLoading() {
        if ($loading) {
            $loading.show();
        }
    }

    function hideLoading() {
        if ($loading) {
            $loading.hide();
        }
    }

    function activeTab() {
        return $taskContentInner.find('li.layui-this').first();
    }

    function activeAppId() {
        return activeTab().attr('lay-id') || '0';
    }

    function activePanel() {
        var id = activeTab().attr('data-panel-id') || SINGLE_PANEL_ID;
        return $('#' + id);
    }

    function rememberHomeApp() {
        var $first = $taskContentInner.find('li').first();
        homeApp.url = $first.attr('app-url') || homeApp.url;
        homeApp.id = $first.attr('lay-id') || $first.attr('app-id') || homeApp.id;
        homeApp.name = $first.attr('app-name') || $.trim($first.text()) || homeApp.name;
    }

    function syncAddress(url) {
        if (!url || !window.history || !history.replaceState) {
            return;
        }
        try {
            var target = toAppUrl(url);
            if (!isWorkspacePage(target)) {
                return;
            }
            var current = window.location.pathname + window.location.search + window.location.hash;
            if (current !== target) {
                history.replaceState({adminWorkspace: true, url: target}, '', target);
            }
        } catch (e) {
            // Keep workspace navigation alive even if a legacy URL cannot be normalized.
        }
    }

    function executableScript(script) {
        var type = (script.getAttribute('type') || '').toLowerCase().replace(/\s+/g, '');
        if (script.getAttribute('src')) {
            return type === '' ||
                type === 'text/javascript' ||
                type === 'application/javascript' ||
                type === 'application/ecmascript' ||
                type === 'module';
        }
        return type === '' ||
            type === 'text/javascript' ||
            type === 'application/javascript' ||
            type === 'application/ecmascript' ||
            type === 'module';
    }

    function scriptSrc(script, pageUrl) {
        var src = script.getAttribute('src');
        if (!src) {
            return '';
        }
        return new URL(src, new URL(pageUrl, window.location.origin)).toString();
    }

    function isCommonScript(src) {
        return /\/jquery(?:-|\.|_)/i.test(src) ||
            /\/jquery-migrate/i.test(src) ||
            /\/wind\.js/i.test(src) ||
            /\/layui\.js/i.test(src) ||
            /\/admin-layui\.js/i.test(src) ||
            /\/admin-workspace\.js/i.test(src) ||
            /\/admin\.js/i.test(src);
    }

    function extractPage(html) {
        var doc = new DOMParser().parseFromString(html, 'text/html');
        var body = doc.body || doc.documentElement;
        var scripts = Array.prototype.slice.call(body.querySelectorAll('script')).filter(executableScript);

        scripts.forEach(function (script) {
            if (script.parentNode) {
                script.parentNode.removeChild(script);
            }
        });

        var content = body.querySelector('.wrap') ||
            body.querySelector('.container-fluid') ||
            body.querySelector('.container') ||
            body.querySelector('.content') ||
            body;

        return {
            title: doc.title || '',
            html: content === body ? body.innerHTML : content.outerHTML,
            scripts: scripts
        };
    }

    function runScripts(scripts, pageUrl) {
        var chain = $.Deferred().resolve().promise();

        $.each(scripts, function (_, script) {
            var src = scriptSrc(script, pageUrl);
            if (src) {
                if (isCommonScript(src)) {
                    return;
                }
                chain = chain.then(function () {
                    if (loadedScripts[src]) {
                        return $.Deferred().resolve().promise();
                    }
                    loadedScripts[src] = true;
                    return $.ajax({
                        url: src,
                        dataType: 'script',
                        cache: true
                    });
                });
                return;
            }

            var code = script.text || script.textContent || '';
            if ($.trim(code) === '') {
                return;
            }
            chain = chain.then(function () {
                $.globalEval(code);
            });
        });

        return chain;
    }

    function renderPanel($panel, url, html) {
        var page = extractPage(html);
        $panel.html(page.html);
        return runScripts(page.scripts, url).always(function () {
            $panel.data('loaded', true).attr('data-url', toAppUrl(url));
            if (window.layuiAdmin) {
                window.layuiAdmin.render($panel);
            }
            $panel.scrollTop(0);
            $(document).trigger('adminWorkspace:loaded', [$panel, url, page.title]);
        });
    }

    function showPanelError($panel, url, xhr) {
        var status = xhr && xhr.status ? xhr.status : '';
        $panel.html(
            '<div class="layui-admin-error">' +
            '<h2>页面加载失败</h2>' +
            '<p>' + escapeHtml(status ? ('HTTP ' + status + ' - ' + url) : url) + '</p>' +
            '<button type="button" class="layui-btn layui-btn-normal" data-workspace-retry="' + escapeHtml(url) + '">重试</button>' +
            '</div>'
        );
    }

    function loadPanel($panel, url, options) {
        options = options || {};
        url = toAppUrl(url);
        showLoading();
        $panel.addClass('loading');

        return $.ajax({
            url: url,
            type: options.method || 'GET',
            data: options.data || undefined,
            dataType: 'html',
            headers: requestHeaders()
        }).done(function (html) {
            if (typeof html === 'string' && /<form[^>]+doLogin|ADMIN_CENTER|login-modern/i.test(html) && /captcha|password/i.test(html)) {
                window.location.href = url;
                return;
            }
            renderPanel($panel, url, html);
            if (options.updateHash !== false) {
                syncAddress(url);
            }
        }).fail(function (xhr) {
            showPanelError($panel, url, xhr);
        }).always(function () {
            $panel.removeClass('loading');
            hideLoading();
        });
    }

    function calcTaskContentWidth() {
        $taskContentInner.find('li').not(':first').remove();
        $('#content > .layui-tabs-item').not(':first').remove();
    }

    function createTab(url, appId, appname) {
        var $tab = $taskContentInner.find('li').first();
        if (!$tab.length) {
            $taskContentInner.append('<li class="layui-this noclose" lay-closable="false" data-panel-id="' + SINGLE_PANEL_ID + '"></li>');
            $tab = $taskContentInner.find('li').first();
        }

        var $item = $('#content > .layui-tabs-item').first();
        if (!$item.length) {
            $('#content').append('<div class="layui-tabs-item layui-show"></div>');
            $item = $('#content > .layui-tabs-item').first();
        }

        var $panel = $('#' + SINGLE_PANEL_ID);
        if (!$panel.length) {
            $item.html('<div id="' + SINGLE_PANEL_ID + '" class="app-panel active"></div>');
            $panel = $('#' + SINGLE_PANEL_ID);
        }

        $taskContentInner.find('li').not($tab).remove();
        $('#content > .layui-tabs-item').not($item).remove();

        $tab.attr({
            'app-id': appId,
            'app-url': url,
            'app-name': appname,
            'lay-id': appId,
            'lay-closable': 'false',
            'data-panel-id': SINGLE_PANEL_ID
        }).addClass('layui-this noclose').text(appname);
        $item.attr('lay-id', appId).addClass('layui-show');
        $panel.addClass('active');
        return $tab;
    }

    function activateTab(appId) {
        var $tab = $taskContentInner.find('li').first();
        if (!$tab.length) {
            return;
        }
        appId = String(appId || $tab.attr('lay-id') || '0');
        $taskContentInner.find('.layui-this').removeClass('layui-this');
        $tab.attr('lay-id', appId).addClass('layui-this');
        $('#content > .layui-tabs-item').removeClass('layui-show');
        $('#content > .layui-tabs-item').first().attr('lay-id', appId).addClass('layui-show');
        syncNavigationForUrl($tab.attr('app-url'), appId);
        syncAddress($tab.attr('app-url'));
    }

    function closeapp($this) {
        if (homeApp.url) {
            openapp(homeApp.url, homeApp.id, homeApp.name, true);
            return;
        }
        reloadCurrentPanel();
    }

    function openapp(url, appId, appname, refresh) {
        url = toAppUrl(url);
        appId = String(appId || url);
        appname = appname || url;

        var $app = createTab(url, appId, appname);
        $app.attr('app-url', url).attr('app-name', appname);
        activateTab(appId);
        calcTaskContentWidth();

        var $panel = activePanel();
        if (refresh === true || !$panel.data('loaded') || $panel.attr('data-url') !== url) {
            loadPanel($panel, url);
        } else {
            syncAddress(url);
        }
    }

    function reloadCurrentPanel() {
        var $panel = activePanel();
        var url = activeTab().attr('app-url') || $panel.attr('data-url');
        if ($panel.length && url) {
            loadPanel($panel, url);
        }
    }

    function loadInCurrentPanel(url, options) {
        options = options || {};
        var $panel = activePanel();
        if (!$panel.length) {
            return;
        }
        url = toAppUrl(url);
        syncActiveTabMeta(url);
        loadPanel($panel, url, options);
    }

    function ajaxMessage(text, type, callback) {
        if (window.layuiAdmin) {
            window.layuiAdmin.message(text, type, callback);
            return;
        }
        if (callback) {
            callback();
        }
    }

    function ajaxConfirm(text, callback) {
        if (window.layuiAdmin) {
            window.layuiAdmin.confirm(text, callback);
            return;
        }
        if (window.confirm(text || '您确定要进行此操作吗？')) {
            callback();
        }
    }

    function handleJsonResult(data, refresh) {
        data = data || {};
        var ok = data.code === 1 || data.code === '1';
        ajaxMessage(data.msg || (ok ? '操作成功' : '操作失败'), ok ? 'success' : 'error', function () {
            if (!ok) {
                return;
            }
            if (refresh === false || refresh === 'false' || refresh === 0 || refresh === '0') {
                return;
            }
            if (data.url && isWorkspacePage(data.url)) {
                loadInCurrentPanel(data.url);
                return;
            }
            reloadCurrentPanel();
        });
    }

    function appendSubmitterData(payload, submitter, hasFile) {
        var button = submitter && submitter.jquery ? submitter.get(0) : submitter;
        if (!button || !button.name || button.disabled) {
            return payload;
        }

        if (hasFile) {
            payload.append(button.name, button.value || '');
            return payload;
        }

        var data = {};
        data[button.name] = button.value || '';
        return payload ? payload + '&' + $.param(data) : $.param(data);
    }

    function ajaxSubmitForm(form, submitter) {
        var $form = $(form);
        var $btn = $(submitter || $form.find('.js-ajax-submit').get(0));
        var url = $btn.data('action') || $form.attr('action') || activeTab().attr('app-url');
        var method = $btn.data('method') || $form.attr('method') || 'post';
        var hasFile = $form.find('input[type="file"]').length > 0 || /multipart\/form-data/i.test($form.attr('enctype') || '');
        var payload = hasFile ? new FormData(form) : $form.serialize();
        payload = appendSubmitterData(payload, $btn.get(0), hasFile);

        $btn.prop('disabled', true).addClass('disabled');
        $.ajax({
            url: url,
            type: method,
            data: payload,
            dataType: 'json',
            headers: requestHeaders(),
            processData: !hasFile,
            contentType: hasFile ? false : 'application/x-www-form-urlencoded; charset=UTF-8'
        }).done(function (data) {
            handleJsonResult(data, $btn.data('refresh'));
        }).always(function () {
            $btn.prop('disabled', false).removeClass('disabled');
        });
    }

    function submitContentForm(form, submitter) {
        var $form = $(form);
        if ($form.hasClass('js-ajax-form')) {
            ajaxSubmitForm(form, submitter);
            return;
        }

        var action = $form.attr('action') || activeTab().attr('app-url') || window.location.pathname;
        var method = ($form.attr('method') || 'get').toUpperCase();
        if (isExportUrl(action)) {
            var oldTarget = form.target;
            form.target = '_blank';
            nativeSubmit.call(form);
            form.target = oldTarget;
            return;
        }

        if (method === 'GET') {
            var parsed = toUrl(action);
            var query = $form.serialize();
            if (query) {
                parsed.search = parsed.search ? (parsed.search.replace(/^\?/, '') + '&' + query) : query;
            }
            loadInCurrentPanel(parsed.pathname + parsed.search + parsed.hash);
            return;
        }

        loadInCurrentPanel(action, {
            method: method,
            data: $form.serialize()
        });
    }

    function handleAjaxAction($link) {
        var url = $link.data('href') || $link.attr('href');
        var refresh = $link.data('refresh');
        var msg = $link.data('msg') || ($link.hasClass('js-ajax-delete') ? '您确定要删除吗？' : '您确定要进行此操作吗？');
        var method = $link.data('method') || 'post';

        ajaxConfirm(msg, function () {
            $.ajax({
                url: url,
                type: method,
                dataType: 'json',
                headers: requestHeaders()
            }).done(function (data) {
                handleJsonResult(data, refresh);
            });
        });
    }

    function menuTitle(menu) {
        return menu && menu.title ? menu.title : '';
    }

    function renderMenuItem(menu, level) {
        var children = menu.children || [];
        var icon = level === 0 ? '<i class="fa fa-' + escapeHtml(menu.icon || 'circle-o') + '"></i>' : '';
        if (children.length) {
            return '<li class="layui-menu-item-group layui-menu-item-up">' +
                '<div class="layui-menu-body-title">' + icon + '<span>' + escapeHtml(menuTitle(menu)) + '</span></div>' +
                '<ul>' + renderMenuItems(children, level + 1) + '</ul>' +
                '</li>';
        }
        return '<li class="admin-menu-leaf" data-menu-id="' + escapeHtml(menu.id) + '" data-menu-url="' + escapeHtml(menu.url) + '" data-menu-title="' + escapeHtml(menuTitle(menu)) + '">' +
            '<div class="layui-menu-body-title">' + icon + '<span>' + escapeHtml(menuTitle(menu)) + '</span></div>' +
            '</li>';
    }

    function renderMenuItems(items, level) {
        return $.map(items || [], function (item) {
            return renderMenuItem(item, level);
        }).join('');
    }

    function renderSideMenu(rootIndex) {
        var root = menuTree[rootIndex] || menuTree[0] || null;
        if (!root) {
            $('#admin-side-title').text('');
            $('#admin-side-menu').empty();
            return;
        }
        var items = root.children && root.children.length ? root.children : [root];
        $('#admin-side-title').text(menuTitle(root));
        $('#admin-side-menu').html(renderMenuItems(items, 0));
        $('.admin-top-nav .layui-nav-item').removeClass('layui-this');
        $('.admin-top-nav [data-root-index="' + rootIndex + '"]').closest('li').addClass('layui-this');
        scrollTopNavIntoView(rootIndex);
        if (elementApi) {
            elementApi.render('nav', 'admin-top-nav');
        }
    }

    function renderHomeMenu() {
        var title = homeApp.name || '首页';
        var url = homeApp.url || '';
        var id = homeApp.id || '0';
        $('#admin-side-title').text(title);
        $('#admin-side-menu').html(
            '<li class="admin-menu-leaf layui-menu-item-checked" data-menu-id="' + escapeHtml(id) + '" data-menu-url="' + escapeHtml(url) + '" data-menu-title="' + escapeHtml(title) + '">' +
            '<div class="layui-menu-body-title"><i class="fa fa-home"></i><span>' + escapeHtml(title) + '</span></div>' +
            '</li>'
        );
        $('.admin-top-nav .layui-nav-item').removeClass('layui-this');
        $('.admin-top-nav [data-home-url]').closest('li').addClass('layui-this');
        var nav = $('.admin-top-nav').get(0);
        if (nav) {
            nav.scrollLeft = 0;
        }
        if (elementApi) {
            elementApi.render('nav', 'admin-top-nav');
        }
    }

    function scrollTopNavIntoView(rootIndex) {
        var nav = $('.admin-top-nav').get(0);
        var item = $('.admin-top-nav [data-root-index="' + rootIndex + '"]').closest('li').get(0);
        if (!nav || !item) {
            return;
        }
        var left = item.offsetLeft;
        var right = left + item.offsetWidth;
        if (left < nav.scrollLeft) {
            nav.scrollLeft = left - 12;
            return;
        }
        if (right > nav.scrollLeft + nav.clientWidth) {
            nav.scrollLeft = right - nav.clientWidth + 12;
        }
    }

    function highlightMenu(appId) {
        $('#admin-side-menu .layui-menu-item-checked').removeClass('layui-menu-item-checked');
        var $leaf = $('#admin-side-menu [data-menu-id="' + appId + '"]');
        if (!$leaf.length) {
            return;
        }
        $leaf.addClass('layui-menu-item-checked');
        $leaf.parents('.layui-menu-item-group').removeClass('layui-menu-item-up').addClass('layui-menu-item-down');
    }

    function findMenuPathByUrl(items, url, path) {
        url = toAppUrl(url || '');
        path = path || [];
        var found = null;
        $.each(items || [], function (_, item) {
            var currentPath = path.concat(item);
            if (item.url && menuUrlMatches(item.url, url)) {
                found = currentPath;
                return false;
            }
            found = findMenuPathByUrl(item.children || [], url, currentPath);
            return !found;
        });
        return found;
    }

    function menuUrlMatches(menuUrl, targetUrl) {
        try {
            var menu = toUrl(menuUrl);
            var target = toUrl(targetUrl);
            var menuFull = menu.pathname + menu.search + menu.hash;
            var targetFull = target.pathname + target.search + target.hash;
            if (menuFull === targetFull) {
                return true;
            }
            return menu.pathname === target.pathname && !menu.search && !menu.hash;
        } catch (e) {
            return false;
        }
    }

    function findMenuPath(url) {
        var found = null;
        $.each(menuTree || [], function (rootIndex, root) {
            found = findMenuPathByUrl([root], url, []);
            if (found) {
                found.rootIndex = rootIndex;
                return false;
            }
            return true;
        });
        return found;
    }

    function syncNavigationForUrl(url, appId) {
        if (isHomeUrl(url) || String(appId || '') === String(homeApp.id || '0')) {
            renderHomeMenu();
            return;
        }

        var path = findMenuPath(url);
        if (path && path.length) {
            renderSideMenu(path.rootIndex || 0);
            highlightMenu(path[path.length - 1].id);
            return;
        }

        $('#admin-side-menu .layui-menu-item-checked').removeClass('layui-menu-item-checked');
    }

    function syncActiveTabMeta(url) {
        var $tab = activeTab();
        var appId = $tab.attr('lay-id') || $tab.attr('app-id') || url;
        var appName = $tab.attr('app-name') || $.trim($tab.text()) || url;
        var path;
        var leaf;

        if (isHomeUrl(url)) {
            appId = homeApp.id || '0';
            appName = homeApp.name || '首页';
        } else {
            path = findMenuPath(url);
            if (path && path.length) {
                leaf = path[path.length - 1];
                appId = leaf.id;
                appName = menuTitle(leaf);
            }
        }

        $tab.attr({
            'app-id': appId,
            'app-url': url,
            'app-name': appName,
            'lay-id': appId
        }).text(appName);
        $('#content > .layui-tabs-item').first().attr('lay-id', appId);
        syncNavigationForUrl(url, appId);
    }

    function openUrlFromMenu(url, refresh) {
        if (isHomeUrl(url)) {
            openapp(homeApp.url, homeApp.id, homeApp.name, refresh);
            return;
        }
        var path = findMenuPath(url);
        if (!path || !path.length) {
            openapp(url, url, url, refresh);
            return;
        }
        renderSideMenu(path.rootIndex || 0);
        var leaf = path[path.length - 1];
        openapp(leaf.url, leaf.id, menuTitle(leaf), refresh);
    }

    function consumeWorkspaceOpenUrl() {
        var url = '';
        try {
            url = window.sessionStorage.getItem('ADMIN_WORKSPACE_OPEN_URL') || '';
            window.sessionStorage.removeItem('ADMIN_WORKSPACE_OPEN_URL');
        } catch (e) {
        }
        if (!url || !isWorkspacePage(url)) {
            return '';
        }
        var shellPath = window.location.pathname + window.location.search;
        if (toAppUrl(url) === shellPath || /\/admin(?:\/index\/index)?(?:\.html)?$/i.test(toUrl(url).pathname)) {
            return '';
        }
        return url;
    }

    function openMenuLeaf($leaf) {
        var url = $leaf.data('menu-url');
        var id = $leaf.data('menu-id');
        var title = $leaf.data('menu-title');
        if (!url) {
            return;
        }
        openapp(url, id, title, true);
    }

    function bindWorkspaceEvents() {
        $('.admin-top-nav').on('click', '[data-home-url]', function () {
            openapp(homeApp.url || $(this).data('home-url'), homeApp.id, homeApp.name || $(this).data('home-title'), true);
            return false;
        });

        $('.admin-top-nav').on('click', '[data-root-index]', function () {
            renderSideMenu(parseInt($(this).data('root-index'), 10) || 0);
        });

        $('.admin-top-nav').on('wheel', function (event) {
            var original = event.originalEvent || {};
            if (this.scrollWidth <= this.clientWidth) {
                return;
            }
            this.scrollLeft += original.deltaX || original.deltaY || 0;
            event.preventDefault();
        });

        $('#admin-side-menu').on('click', '.admin-menu-leaf', function () {
            openMenuLeaf($(this));
            return false;
        });

        $('#admin-side-menu').on('click', '.layui-menu-item-group > .layui-menu-body-title', function () {
            $(this).parent().toggleClass('layui-menu-item-down layui-menu-item-up');
            return false;
        });

        $('#content').on('click', 'a', function (event) {
            var $link = $(this);
            var href = $link.attr('href');

            if ($link.is('.js-ajax-delete,.js-ajax-dialog-btn,.js-ajax-btn')) {
                event.preventDefault();
                event.stopImmediatePropagation();
                handleAjaxAction($link);
                return false;
            }

            if (!href || $link.attr('target') === '_blank' || /^javascript:/i.test(href) || href.charAt(0) === '#') {
                return;
            }

            if (isWorkspacePage(href) && !isExportUrl(href)) {
                event.preventDefault();
                loadInCurrentPanel(href);
                return false;
            }
        });

        $('#content').on('click', '.js-ajax-submit', function () {
            $(this).closest('form').data('workspaceSubmitter', this);
        });

        $('#content').on('submit', 'form', function (event) {
            event.preventDefault();
            submitContentForm(this, $(this).data('workspaceSubmitter'));
            return false;
        });

        $('#content').on('click', '[data-workspace-retry]', function () {
            loadInCurrentPanel($(this).data('workspace-retry'));
        });
    }

    if (nativeSubmit && !window.__adminWorkspaceSubmitPatched) {
        window.__adminWorkspaceSubmitPatched = true;
        window.HTMLFormElement.prototype.submit = function () {
            if ($(this).closest('.app-panel').length) {
                submitContentForm(this, $(this).data('workspaceSubmitter'));
                return;
            }
            nativeSubmit.call(this);
        };
    }

    window.openapp = openapp;
    window.close_current_app = function () {
        closeapp(activeTab());
    };
    window.reloadPage = function (win) {
        if (!win || win === window) {
            reloadCurrentPanel();
            return;
        }
        win.location.href = win.location.pathname + win.location.search;
    };
    window.redirect = function (url) {
        if (isWorkspacePage(url)) {
            loadInCurrentPanel(url);
            return;
        }
        window.location.href = url;
    };

    function renderLayuiShell() {
        if (tabsApi && tabsApi.render) {
            tabsApi.render();
        }
        if (elementApi) {
            elementApi.render('nav');
            elementApi.render('tab');
        }
    }

    function bootWorkspace() {
        renderLayuiShell();
        if (workspaceBooted) {
            return;
        }
        workspaceBooted = true;
        rememberHomeApp();
        renderHomeMenu();
        bindWorkspaceEvents();
        calcTaskContentWidth();

        var pendingUrl = consumeWorkspaceOpenUrl();
        if (pendingUrl) {
            openUrlFromMenu(pendingUrl, true);
            return;
        }

        var $first = $taskContentInner.find('li.layui-this').first();
        if ($first.length) {
            openapp($first.attr('app-url'), $first.attr('lay-id') || '0', $first.attr('app-name'), true);
        }
    }

    $(function () {
        $taskContentInner = $('#task-content-inner');
        $loading = $('#loading');

        bootWorkspace();

        if (window.layui && layui.use) {
            layui.use(['element', 'tabs'], function () {
                elementApi = layui.element;
                tabsApi = layui.tabs;
                bootWorkspace();
            });
        }
    });
})(window, window.jQuery);
