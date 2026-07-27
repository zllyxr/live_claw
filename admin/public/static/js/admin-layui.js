;(function (window, $) {
    if (!$) {
        return;
    }

    var layuiReady = false;
    var layerApi = null;
    var formApi = null;
    var laydateApi = null;
    var elementApi = null;
    var tableApi = null;
    var tableIndex = 0;

    function isSuccess(data) {
        return data && (data.code === 1 || data.code === '1');
    }

    function message(text, type, callback) {
        text = text || '';
        type = type || 'info';
        if (layerApi) {
            var iconMap = {
                success: 1,
                error: 2,
                warning: 0,
                information: 0,
                info: 0
            };
            layerApi.msg(text, {
                icon: iconMap[type] || 0,
                time: 1200,
                shade: type === 'success' || type === 'error' ? 0.08 : 0
            }, callback || function () {});
            return;
        }
        if (callback) {
            callback();
        }
    }

    function confirm(text, yes, no) {
        text = text || '您确定要进行此操作吗？';
        if (layerApi) {
            layerApi.confirm(text, {
                title: '操作确认',
                btn: ['确定', '取消']
            }, function (index) {
                layerApi.close(index);
                if (yes) {
                    yes();
                }
            }, function () {
                if (no) {
                    no();
                }
            });
            return;
        }
        if (window.confirm(text) && yes) {
            yes();
        }
    }

    function renderDates(scope) {
        if (!laydateApi) {
            return;
        }
        var $scope = scope ? $(scope) : $(document);

        var configs = [
            ['input.js-date', {type: 'date'}],
            ['input.js-bootstrap-date', {type: 'date'}],
            ['input.js-datetime', {type: 'datetime', format: 'yyyy-MM-dd HH:mm'}],
            ['input.js-bootstrap-datetime', {type: 'datetime', format: 'yyyy-MM-dd HH:mm'}],
            ['input.js-year', {type: 'year'}],
            ['input.js-bootstrap-year', {type: 'year'}],
            ['input.js-bootstrap-year-month', {type: 'month'}]
        ];

        $.each(configs, function (_, item) {
            $scope.find(item[0]).addBack(item[0]).each(function () {
                var elem = this;
                if ($(elem).data('layui-laydate-bound')) {
                    return;
                }
                $(elem).data('layui-laydate-bound', true);
                laydateApi.render($.extend({elem: elem, trigger: 'click'}, item[1]));
            });
        });
    }

    function normalizeForms(scope) {
        var $scope = scope ? $(scope) : $(document);
        $scope.find('form').addBack('form').each(function () {
            var $form = $(this);
            if (!$form.hasClass('layui-form')) {
                $form.addClass('layui-form');
            }
        });
        $scope.find('input.form-control').addBack('input.form-control').addClass('layui-input');
        $scope.find('textarea.form-control').addBack('textarea.form-control').addClass('layui-textarea');
        $scope.find('select.form-control').addBack('select.form-control').addClass('layui-select');
        $scope.find('.btn').addBack('.btn').each(function () {
            var $btn = $(this);
            if (!$btn.hasClass('layui-btn')) {
                $btn.addClass('layui-btn');
            }
            if ($btn.hasClass('btn-primary') || $btn.hasClass('btn-success')) {
                $btn.addClass('layui-btn-normal');
            } else if ($btn.hasClass('btn-danger')) {
                $btn.addClass('layui-btn-danger');
            } else if ($btn.hasClass('btn-warning')) {
                $btn.addClass('layui-btn-warm');
            } else if (!$btn.hasClass('layui-btn-primary')) {
                $btn.addClass('layui-btn-primary');
            }
        });
    }

    function renderForm(scope) {
        normalizeForms(scope);
        if (formApi) {
            formApi.render();
        }
    }

    function tableCellHtml(cell) {
        return $.trim($(cell).html() || '');
    }

    function staticColumnWidth(title, index, total, definedWidth) {
        title = $.trim(title || '');
        if (!title) {
            return Math.max(definedWidth || 0, 52);
        }
        if (/^(操作|actions)$/i.test(title)) {
            return Math.max(definedWidth || 0, 340);
        }
        if (/^(id|ID)$/.test(title)) {
            return Math.max(definedWidth || 0, 84);
        }
        if (/图标\s*\/\s*游戏|图标.*游戏/.test(title)) {
            return Math.max(definedWidth || 0, 220);
        }
        if (/数据源|博易标识|本地开奖|间隔\s*\/\s*封盘/.test(title)) {
            return Math.max(definedWidth || 0, 130);
        }
        if (/当前期/.test(title)) {
            return Math.max(definedWidth || 0, 150);
        }
        if (/限额/.test(title)) {
            return Math.max(definedWidth || 0, 150);
        }
        if (/头像|图片|封面|图标/.test(title)) {
            return Math.max(definedWidth || 0, 72);
        }
        if (/^动画$/.test(title)) {
            return Math.max(definedWidth || 0, 150);
        }
        if (/动画类型|动画时长/.test(title)) {
            return Math.max(definedWidth || 0, 100);
        }
        if (/状态|开关|启用|禁用/.test(title)) {
            return Math.max(definedWidth || 0, 92);
        }
        if (/^类型$|^标识$/.test(title)) {
            return Math.max(definedWidth || 0, 100);
        }
        if (/时间|日期/.test(title)) {
            return Math.max(definedWidth || 0, 150);
        }
        if (/IP/i.test(title)) {
            return Math.max(definedWidth || 0, 130);
        }
        if (/手机|电话|用户名|账号/.test(title)) {
            return Math.max(definedWidth || 0, 120);
        }
        if (/昵称|名称|标题|国家|地区/.test(title)) {
            return Math.max(definedWidth || 0, 110);
        }
        if (/余额|累计|收入|消费|金额|星币|人民币|收益|数量/.test(title)) {
            return Math.max(definedWidth || 0, 112);
        }
        if (/所需点数|价格/.test(title)) {
            return Math.max(definedWidth || 0, 100);
        }
        if (/邀请码|注册设备|设备来源|发布者/.test(title)) {
            return Math.max(definedWidth || 0, 120);
        }
        if (definedWidth) {
            return definedWidth;
        }
        return Math.min(Math.max(title.length * 16 + 34, 90), 160);
    }

    function staticTableAvailableWidth($table) {
        var $container = $table.closest('.table-responsive');
        var width = $container.innerWidth();
        if (!width) {
            width = $table.closest('.wrap,.container,.container-fluid,.app-panel,#content,body').innerWidth();
        }
        return Math.floor(width || 0);
    }

    function staticColumnStretchWeight(col) {
        var title = $.trim(col.title || '');
        if (!title || col.fixed || /^(id|ID|操作|actions)$/.test(title) || /头像|图片|封面|图标|状态|开关|排序/.test(title)) {
            return 0;
        }
        if (/描述|简介|内容|说明|备注/.test(title)) {
            return 4;
        }
        if (/名称|标题|昵称|用户名|账号/.test(title)) {
            return 3;
        }
        if (/英文|中文/.test(title)) {
            return 2;
        }
        return 1;
    }

    function fillStaticTableWidth($table, cols) {
        var availableWidth = staticTableAvailableWidth($table);
        var totalWidth = 0;
        var stretchWeight = 0;
        var stretchCols = [];
        var extra;

        if (availableWidth < 520) {
            return;
        }

        $.each(cols, function (_, col) {
            totalWidth += col.width || 80;
            var weight = staticColumnStretchWeight(col);
            if (weight > 0) {
                stretchCols.push({
                    col: col,
                    weight: weight
                });
                stretchWeight += weight;
            }
        });

        extra = availableWidth - totalWidth - 2;
        if (extra <= 0 || !stretchWeight) {
            return;
        }

        $.each(stretchCols, function (index, item) {
            var add = index === stretchCols.length - 1 ? extra : Math.floor(extra * item.weight / stretchWeight);
            item.col.width = (item.col.width || 80) + add;
            extra -= add;
            stretchWeight -= item.weight;
        });
    }

    function buildStaticTable(table, filter) {
        var $table = $(table);
        var fixedActions = String($table.attr('data-fixed-actions') || '').toLowerCase();
        var allowFixedActions = fixedActions !== '0' && fixedActions !== 'false' && fixedActions !== 'no';
        var cols = [];
        var rows = [];
        var headerCells = $table.find('thead tr').last().children('th,td');

        headerCells.each(function (index) {
            var $th = $(this);
            var field = 'c' + index;
            var width = parseInt($th.attr('width'), 10);
            var title = $.trim($th.text());
            var isAction = /^(操作|actions)$/i.test(title);
            var col = {
                field: field,
                title: title,
                width: staticColumnWidth(title, index, headerCells.length, width),
                templet: (function (name, wrapActions) {
                    return function (row) {
                        var html = row[name] || '';
                        return wrapActions ? '<div class="admin-table-actions">' + html + '</div>' : html;
                    };
                })(field, isAction)
            };

            if (isAction && allowFixedActions) {
                col.fixed = 'right';
                col.align = 'left';
            } else if (!title || /头像|图片|封面|图标|状态/.test(title)) {
                col.align = 'center';
            }
            if ($th.attr('align')) {
                col.align = $th.attr('align');
            }
            cols.push(col);
        });

        $table.find('tbody tr').each(function (rowIndex) {
            var row = {
                LAY_TABLE_INDEX: rowIndex
            };
            $(this).children('td,th').each(function (cellIndex) {
                row['c' + cellIndex] = tableCellHtml(this);
            });
            rows.push(row);
        });

        if (!cols.length) {
            return false;
        }

        if (!$table.attr('id')) {
            $table.attr('id', filter + '_source');
        }

        fillStaticTableWidth($table, cols);

        $table.hide();
        tableApi.render({
            elem: table,
            id: filter,
            cols: [cols],
            data: rows,
            page: false,
            limit: Math.max(rows.length, 1),
            skin: $table.attr('lay-skin') || 'line',
            even: true,
            cellMinWidth: 80,
            text: {
                none: '暂无数据'
            },
            done: function () {
                $table.hide();
                renderForm($table.closest('.app-panel,.wrap,body'));
            }
        });
        $table.hide();

        return true;
    }

    function enhanceTables(scope) {
        var $scope = scope ? $(scope) : $(document);
        $scope.find('.table,.layui-table').addBack('.table,.layui-table').each(function () {
            var $table = $(this);
            if (!$table.parent().hasClass('table-responsive') && !$table.closest('.layui-table-view').length) {
                $table.wrap('<div class="table-responsive"></div>');
            }
        });

        if (!tableApi) {
            return;
        }

        $scope.find('table.js-layui-table').addBack('table.js-layui-table').each(function () {
            var table = this;
            var $table = $(table);
            if ($table.data('layui-table-inited') || $table.closest('.layui-table-view').length || !$table.find('thead th').length) {
                return;
            }

            var filter = $table.attr('lay-filter');
            if (!filter) {
                filter = 'admin_table_' + (++tableIndex);
                $table.attr('lay-filter', filter);
            }

            $table.data('layui-table-inited', true);
            try {
                buildStaticTable(table, filter);
            } catch (e) {
                $table.data('layui-table-inited', false);
                $table.show();
            }
        });
    }

    function activateLegacyTab($link) {
        var target = $link.attr('href');
        if (!target || target.charAt(0) !== '#') {
            return;
        }

        var pane = document.getElementById(target.substring(1));
        if (!pane) {
            return;
        }

        var $scope = $link.closest('.wrap, .container, .container-fluid, body');
        var $nav = $link.closest('.nav-tabs,.layui-tab-title,.layui-tabs-header');
        var $pane = $(pane);
        var $content = $pane.closest('.tab-content,.layui-tab-content,.layui-tabs-body');

        $nav.find('> li').removeClass('active layui-this');
        $link.closest('li').addClass($nav.hasClass('nav-tabs') ? 'active' : 'layui-this');

        if ($content.length) {
            $content.find('> .tab-pane,> .layui-tab-item,> .layui-tabs-item').removeClass('active layui-show').hide();
        } else {
            $scope.find('.tab-pane,.layui-tab-item,.layui-tabs-item').removeClass('active layui-show').hide();
        }

        $pane.addClass($pane.hasClass('layui-tab-item') || $pane.hasClass('layui-tabs-item') ? 'layui-show' : 'active').show();
        if (formApi) {
            formApi.render();
        }
    }

    function activateNativeTab($li) {
        var $nav = $li.parent();
        var $tab = $nav.closest('.layui-tab,.layui-tabs');
        var $content = $tab.children('.layui-tab-content,.layui-tabs-body').first();
        var layId = $li.attr('lay-id');
        var $items;
        var $target;

        if ($li.find('> a[data-toggle="tab"]').length || $nav.is('#task-content-inner')) {
            return;
        }

        if (!$content.length) {
            $content = $tab.find('.layui-tab-content,.layui-tabs-body').first();
        }
        if (!$content.length) {
            return;
        }

        $items = $content.children('.layui-tab-item,.layui-tabs-item');
        if (!$items.length) {
            return;
        }

        if (layId) {
            $target = $items.filter(function () {
                return this.id === layId || $(this).attr('lay-id') === layId;
            }).first();
        }
        if (!$target || !$target.length) {
            $target = $items.eq($li.index());
        }
        if (!$target.length) {
            return;
        }

        $nav.children('li').removeClass('layui-this');
        $li.addClass('layui-this');
        $items.removeClass('layui-show').hide();
        $target.addClass('layui-show').show();
        if (formApi) {
            formApi.render();
        }
    }

    function bindNativeTabs(scope) {
        var $scope = scope ? $(scope) : $(document);

        $('body')
            .off('click.layuiAdminNativeTabs', '#content .layui-tab-title > li,#content .layui-tabs-header > li')
            .on('click.layuiAdminNativeTabs', '#content .layui-tab-title > li,#content .layui-tabs-header > li', function () {
                var li = this;
                window.setTimeout(function () {
                    activateNativeTab($(li));
                }, 0);
            });

        $scope.find('.layui-tab-title > li.layui-this,.layui-tabs-header > li.layui-this').each(function () {
            activateNativeTab($(this));
        });
    }

    function bindLegacyTabs(scope) {
        var $scope = scope ? $(scope) : $(document);
        var legacyNavSelector = '.nav-tabs,.layui-tab-title,.layui-tabs-header';

        function isLegacyTabNav(nav) {
            return $(nav).find('> li > a[data-toggle="tab"]').length > 0;
        }

        $('body')
            .off('click.layuiAdminTabs', legacyNavSelector + ' a[data-toggle="tab"]')
            .on('click.layuiAdminTabs', '.nav-tabs a[data-toggle="tab"],.layui-tab-title a[data-toggle="tab"],.layui-tabs-header a[data-toggle="tab"]', function (event) {
                event.preventDefault();
                activateLegacyTab($(this));
            });

        $scope.find('.layui-tab-title,.layui-tabs-header').addBack('.layui-tab-title,.layui-tabs-header').each(function () {
            var $nav = $(this);
            if (!isLegacyTabNav($nav)) {
                return;
            }
            var $items = $nav.find('> li');
            if (!$items.filter('.layui-this').length) {
                $items.filter('.active').first().addClass('layui-this');
            }
        });

        $scope.find('.layui-tab-content,.layui-tabs-body').addBack('.layui-tab-content,.layui-tabs-body').each(function () {
            var $content = $(this);
            var $tab = $content.closest('.layui-tab,.layui-tabs');
            if ($tab.length && !isLegacyTabNav($tab.find('> .layui-tab-title,> .layui-tabs-header').first())) {
                return;
            }
            var $items = $content.find('> .layui-tab-item,> .layui-tabs-item');
            if (!$items.filter('.layui-show').length) {
                $items.filter('.active').first().addClass('layui-show');
            }
        });

        $scope.find(legacyNavSelector).addBack(legacyNavSelector).each(function () {
            var $nav = $(this);
            if (!isLegacyTabNav($nav)) {
                return;
            }
            var $active = $nav.find('> li.active > a[data-toggle="tab"],> li.layui-this > a[data-toggle="tab"]');
            if (!$active.length) {
                $active = $nav.find('> li:first > a[data-toggle="tab"]');
            }
            if ($active.length) {
                activateLegacyTab($active);
            }
        });

        if (elementApi) {
            elementApi.render('tab');
        }
        bindNativeTabs(scope);
    }

    function bindAjaxButtons() {
        $('body')
            .off('click.layuiAdminDialog', '.layui-admin-ajax-confirm')
            .on('click.layuiAdminDialog', '.layui-admin-ajax-confirm', function (event) {
                event.preventDefault();
                var $btn = $(this);
                var url = $btn.data('href') || $btn.attr('href');
                var refresh = $btn.data('refresh');
                confirm($btn.data('msg'), function () {
                    $.post(url, function (data) {
                        message(data.msg, isSuccess(data) ? 'success' : 'error', function () {
                            if (isSuccess(data) && (refresh === undefined || refresh)) {
                                if (data.url) {
                                    window.location.href = data.url;
                                } else {
                                    window.location.reload();
                                }
                            }
                        });
                    }, 'json');
                });
            });
    }

    function initLayui() {
        if (!window.layui || layuiReady) {
            return;
        }

        function hydrate() {
            layuiReady = true;
            layerApi = window.layui.layer;
            formApi = window.layui.form;
            laydateApi = window.layui.laydate;
            elementApi = window.layui.element;
            tableApi = window.layui.table;
            renderDates();
            renderForm();
            enhanceTables();
            bindLegacyTabs();
            bindAjaxButtons();
        }

        hydrate();

        if (window.layui.use) {
            window.layui.use(['layer', 'form', 'laydate', 'element', 'table'], function () {
                hydrate();
            });
        }
    }

    $.fn.tooltip = $.fn.tooltip || function () {
        return this.each(function () {
            var $el = $(this);
            var title = $el.attr('title') || $el.data('original-title');
            if (title && !$el.attr('lay-tips')) {
                $el.attr('lay-tips', title);
            }
        });
    };

    $(function () {
        $('[data-bs-toggle="tooltip"]').tooltip();
    });

    if (!window.noty) {
        window.noty = function (options) {
            options = options || {};
            return {
                show: function () {
                    message(options.text, options.type, function () {
                        if (options.callback && typeof options.callback.afterClose === 'function') {
                            options.callback.afterClose();
                        }
                    });
                    return this;
                },
                close: function () {
                    if (layerApi) {
                        layerApi.closeAll('dialog');
                        layerApi.closeAll('loading');
                    }
                }
            };
        };
    }

    window.layuiAdmin = {
        message: message,
        confirm: confirm,
        render: function () {
            var scope = arguments[0];
            renderDates(scope);
            renderForm(scope);
            enhanceTables(scope);
            bindLegacyTabs(scope);
            if (elementApi) {
                elementApi.render();
            }
        },
        layer: function () {
            return layerApi;
        }
    };

    $(function () {
        initLayui();
        // Dynamic workspace pages call layuiAdmin.render(panel) after content is loaded.
        // Observing every DOM mutation here breaks Layui controls, because opening a
        // select/dropdown mutates the generated DOM and would immediately re-render it.
    });
})(window, window.jQuery);
