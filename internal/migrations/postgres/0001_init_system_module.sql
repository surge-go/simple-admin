-- +goose Up
-- 创建角色表
CREATE TABLE sys_role (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(64) NOT NULL,
    is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
    sort INTEGER NOT NULL DEFAULT 0,
    status SMALLINT NOT NULL DEFAULT 1,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT chk_role_status CHECK (status IN (0, 1))
);

-- 创建索引
CREATE INDEX idx_sys_role_deleted_at ON sys_role (deleted_at);

CREATE INDEX idx_sys_role_status ON sys_role (status)
WHERE
    deleted_at IS NULL;

CREATE UNIQUE INDEX idx_sys_role_code ON sys_role (code)
WHERE
    deleted_at IS NULL;

-- 添加表注释
COMMENT ON TABLE sys_role IS '角色表';

-- 添加字段注释
COMMENT ON COLUMN sys_role.id IS '主键ID';
COMMENT ON COLUMN sys_role.code IS '角色编码，全局唯一标识';
COMMENT ON COLUMN sys_role.name IS '角色名称';
COMMENT ON COLUMN sys_role.is_builtin IS '是否内置角色，内置角色不可删除';
COMMENT ON COLUMN sys_role.sort IS '排序号，数字越小越靠前';
COMMENT ON COLUMN sys_role.status IS '状态：1-启用 0-禁用';
COMMENT ON COLUMN sys_role.remark IS '备注说明';
COMMENT ON COLUMN sys_role.created_at IS '创建时间';
COMMENT ON COLUMN sys_role.updated_at IS '更新时间';
COMMENT ON COLUMN sys_role.deleted_at IS '软删除时间';

-- +goose StatementBegin
-- 创建触发器函数：保护内置角色
CREATE OR REPLACE FUNCTION prevent_builtin_role_modification()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.is_builtin = TRUE THEN
        IF TG_OP = 'DELETE' THEN
            RAISE EXCEPTION '内置角色不允许删除';
        END IF;

        -- 禁止修改内置角色的关键字段
        IF NEW.code != OLD.code THEN
            RAISE EXCEPTION '内置角色不允许修改角色编码';
        END IF;

        IF NEW.is_builtin != OLD.is_builtin THEN
            RAISE EXCEPTION '内置角色标识不允许修改';
        END IF;

        IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
            RAISE EXCEPTION '内置角色不允许删除';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- 创建触发器
CREATE TRIGGER trg_sys_role_protect_builtin
    BEFORE UPDATE OR DELETE ON sys_role
    FOR EACH ROW
    EXECUTE FUNCTION prevent_builtin_role_modification();

-- +goose StatementBegin
-- 创建触发器函数：自动更新 updated_at 字段
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- 创建触发器：自动更新 updated_at
CREATE TRIGGER trg_sys_role_update_timestamp
    BEFORE UPDATE ON sys_role
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 创建菜单表
CREATE TABLE sys_menu (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT,
    type VARCHAR(16) NOT NULL,
    route_name VARCHAR(128),
    title VARCHAR(128) NOT NULL,
    path VARCHAR(512),
    component VARCHAR(512),
    link VARCHAR(1024),
    redirect VARCHAR(512),
    icon VARCHAR(128),
    active_icon VARCHAR(128),
    sort INTEGER NOT NULL DEFAULT 0,
    visible BOOLEAN NOT NULL DEFAULT TRUE,
    hide_children_in_menu BOOLEAN NOT NULL DEFAULT FALSE,
    keep_alive BOOLEAN NOT NULL DEFAULT FALSE,
    affix_tab BOOLEAN NOT NULL DEFAULT FALSE,
    status SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT fk_sys_menu_parent FOREIGN KEY (parent_id) REFERENCES sys_menu (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT chk_menu_type CHECK (type IN ('directory', 'page', 'button', 'external')),
    CONSTRAINT chk_menu_button_visible CHECK (
        type != 'button'
        OR visible = FALSE
    ),
    CONSTRAINT chk_menu_external_link CHECK (
        (
            type = 'external'
            AND link IS NOT NULL
            AND link != ''
            AND path IS NULL
            AND component IS NULL
        )
        OR (
            type != 'external'
            AND link IS NULL
        )
    ),
    CONSTRAINT chk_menu_page_component CHECK (
        (
            type = 'page'
            AND component IS NOT NULL
            AND component != ''
        )
        OR (type != 'page')
    ),
    CONSTRAINT chk_menu_status CHECK (status IN (0, 1))
);

-- 创建索引
CREATE INDEX idx_sys_menu_parent_id ON sys_menu (parent_id)
WHERE
    deleted_at IS NULL;

CREATE INDEX idx_sys_menu_parent_deleted ON sys_menu (parent_id, deleted_at);

CREATE INDEX idx_sys_menu_deleted_at ON sys_menu (deleted_at);

CREATE INDEX idx_sys_menu_type ON sys_menu (type)
WHERE
    deleted_at IS NULL;

CREATE UNIQUE INDEX idx_sys_menu_route_name ON sys_menu (route_name)
WHERE
    deleted_at IS NULL
    AND route_name IS NOT NULL;

-- 添加表注释
COMMENT ON TABLE sys_menu IS '菜单权限表';

-- 添加字段注释
COMMENT ON COLUMN sys_menu.id IS '主键ID';
COMMENT ON COLUMN sys_menu.parent_id IS '父菜单ID，NULL表示根节点';
COMMENT ON COLUMN sys_menu.type IS '菜单类型：directory-目录 page-页面 button-按钮 external-外链';
COMMENT ON COLUMN sys_menu.route_name IS '前端路由名称，全局唯一';
COMMENT ON COLUMN sys_menu.title IS '菜单标题/按钮文本';
COMMENT ON COLUMN sys_menu.path IS '路由路径';
COMMENT ON COLUMN sys_menu.component IS '组件路径';
COMMENT ON COLUMN sys_menu.link IS '外链地址';
COMMENT ON COLUMN sys_menu.redirect IS '重定向路径';
COMMENT ON COLUMN sys_menu.icon IS '菜单图标';
COMMENT ON COLUMN sys_menu.active_icon IS '激活状态图标';
COMMENT ON COLUMN sys_menu.sort IS '排序号，数字越小越靠前';
COMMENT ON COLUMN sys_menu.visible IS '是否显示在菜单中，按钮类型通常为FALSE';
COMMENT ON COLUMN sys_menu.hide_children_in_menu IS '是否隐藏子菜单';
COMMENT ON COLUMN sys_menu.keep_alive IS '是否缓存页面';
COMMENT ON COLUMN sys_menu.affix_tab IS '是否固定在标签页';
COMMENT ON COLUMN sys_menu.status IS '状态：1-启用 0-禁用';
COMMENT ON COLUMN sys_menu.created_at IS '创建时间';
COMMENT ON COLUMN sys_menu.updated_at IS '更新时间';
COMMENT ON COLUMN sys_menu.deleted_at IS '软删除时间';

-- 创建触发器：自动更新 sys_menu 的 updated_at
CREATE TRIGGER trg_sys_menu_update_timestamp
    BEFORE UPDATE ON sys_menu
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 创建API资源表
CREATE TABLE sys_api (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT,
    group_name VARCHAR(64),
    name VARCHAR(128) NOT NULL,
    perm_code VARCHAR(128),
    method VARCHAR(8),
    path VARCHAR(512),
    sort INTEGER NOT NULL DEFAULT 0,
    remark VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT fk_sys_api_parent FOREIGN KEY (parent_id) REFERENCES sys_api (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT chk_api_node_type CHECK (
        -- 分组节点（可嵌套）：group_name 有值，method 和 path 必须为 NULL，perm_code 可为空
        (
            group_name IS NOT NULL
            AND group_name != ''
            AND method IS NULL
            AND path IS NULL
        )
        OR
        -- API 节点（叶子节点）：group_name 为 NULL，method、path 和 perm_code 必须有值，parent_id 必须指向分组
        (
            group_name IS NULL
            AND method IS NOT NULL
            AND method != ''
            AND path IS NOT NULL
            AND path != ''
            AND perm_code IS NOT NULL
            AND perm_code != ''
            AND parent_id IS NOT NULL
        )
    ),
    CONSTRAINT chk_api_method CHECK (
        method IS NULL
        OR method IN ('GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS')
    ),
    CONSTRAINT chk_api_perm_code_format CHECK (
        perm_code IS NULL
        OR perm_code ~ '^[a-z][a-z0-9_-]*(:[a-z][a-z0-9_-]*)+$'
    )
);

-- 创建索引
CREATE INDEX idx_sys_api_parent_id ON sys_api (parent_id)
WHERE
    deleted_at IS NULL;

CREATE INDEX idx_sys_api_parent_deleted ON sys_api (parent_id, deleted_at);

CREATE INDEX idx_sys_api_deleted_at ON sys_api (deleted_at);

CREATE INDEX idx_sys_api_sort ON sys_api (sort)
WHERE
    deleted_at IS NULL;

CREATE UNIQUE INDEX idx_sys_api_group_name ON sys_api (group_name)
WHERE
    deleted_at IS NULL
    AND group_name IS NOT NULL;

CREATE UNIQUE INDEX idx_sys_api_perm_code ON sys_api (perm_code)
WHERE
    deleted_at IS NULL
    AND perm_code IS NOT NULL;

CREATE UNIQUE INDEX idx_sys_api_path_method ON sys_api (LOWER(path), method)
WHERE
    deleted_at IS NULL
    AND path IS NOT NULL;

-- 添加表注释
COMMENT ON TABLE sys_api IS 'API资源表，支持多层分组嵌套';

-- 添加字段注释
COMMENT ON COLUMN sys_api.id IS '主键ID';
COMMENT ON COLUMN sys_api.parent_id IS '父节点ID；NULL表示根分组，非NULL时可指向父分组（分组嵌套）或分组（API所属）';
COMMENT ON COLUMN sys_api.group_name IS '分组标识，仅分组节点时有值且全局唯一，支持多层嵌套';
COMMENT ON COLUMN sys_api.name IS '名称：分组节点为分组名称，API节点为接口描述';
COMMENT ON COLUMN sys_api.perm_code IS '权限编码，格式如 system:user:create，全局唯一';
COMMENT ON COLUMN sys_api.method IS 'HTTP方法：GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS，仅API节点时有值';
COMMENT ON COLUMN sys_api.path IS 'API路径，如 /api/v1/user，仅API节点时有值，统一存储为小写';
COMMENT ON COLUMN sys_api.sort IS '排序号，数字越小越靠前';
COMMENT ON COLUMN sys_api.remark IS '备注说明';
COMMENT ON COLUMN sys_api.created_at IS '创建时间';
COMMENT ON COLUMN sys_api.updated_at IS '更新时间';
COMMENT ON COLUMN sys_api.deleted_at IS '软删除时间';

-- 创建触发器：自动更新 sys_api 的 updated_at
CREATE TRIGGER trg_sys_api_update_timestamp
    BEFORE UPDATE ON sys_api
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 创建用户表
CREATE TABLE sys_user (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    password VARCHAR(255) NOT NULL,
    real_name VARCHAR(64),
    nickname VARCHAR(64),
    avatar VARCHAR(512),
    email VARCHAR(255),
    phone VARCHAR(32),
    gender SMALLINT NOT NULL DEFAULT 0,
    status SMALLINT NOT NULL DEFAULT 1,
    last_login_at TIMESTAMP,
    last_login_ip VARCHAR(128),
    locked_until TIMESTAMP,
    remark VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT chk_user_gender CHECK (gender IN (0, 1, 2)),
    CONSTRAINT chk_user_status CHECK (status IN (0, 1))
);

-- 创建唯一索引
CREATE UNIQUE INDEX idx_sys_user_username ON sys_user (username)
WHERE
    deleted_at IS NULL;

CREATE UNIQUE INDEX idx_sys_user_email ON sys_user (email)
WHERE
    deleted_at IS NULL
    AND email IS NOT NULL;

CREATE UNIQUE INDEX idx_sys_user_phone ON sys_user (phone)
WHERE
    deleted_at IS NULL
    AND phone IS NOT NULL;

-- 创建普通索引
CREATE INDEX idx_sys_user_deleted_at ON sys_user (deleted_at);

CREATE INDEX idx_sys_user_status ON sys_user (status)
WHERE
    deleted_at IS NULL;

CREATE INDEX idx_sys_user_locked_until ON sys_user (locked_until)
WHERE
    locked_until IS NOT NULL
    AND deleted_at IS NULL;

-- 添加表注释
COMMENT ON TABLE sys_user IS '用户表';

-- 添加字段注释
COMMENT ON COLUMN sys_user.id IS '主键ID';
COMMENT ON COLUMN sys_user.username IS '用户名，登录账号，全局唯一';
COMMENT ON COLUMN sys_user.password IS '密码，使用bcrypt等算法加密存储';
COMMENT ON COLUMN sys_user.real_name IS '真实姓名';
COMMENT ON COLUMN sys_user.nickname IS '昵称，用于页面显示';
COMMENT ON COLUMN sys_user.avatar IS '头像URL';
COMMENT ON COLUMN sys_user.email IS '邮箱地址，用于找回密码、接收通知';
COMMENT ON COLUMN sys_user.phone IS '手机号，用于找回密码、二次验证';
COMMENT ON COLUMN sys_user.gender IS '性别：0-未知 1-男 2-女';
COMMENT ON COLUMN sys_user.status IS '状态：1-正常 0-禁用';
COMMENT ON COLUMN sys_user.last_login_at IS '最后登录时间';
COMMENT ON COLUMN sys_user.last_login_ip IS '最后登录IP地址';
COMMENT ON COLUMN sys_user.locked_until IS '账号锁定截止时间，NULL表示未锁定，超过此时间自动解锁';
COMMENT ON COLUMN sys_user.remark IS '备注说明';
COMMENT ON COLUMN sys_user.created_at IS '创建时间';
COMMENT ON COLUMN sys_user.updated_at IS '更新时间';
COMMENT ON COLUMN sys_user.deleted_at IS '软删除时间';

-- 创建触发器：自动更新 sys_user 的 updated_at
CREATE TRIGGER trg_sys_user_update_timestamp
    BEFORE UPDATE ON sys_user
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 创建用户角色关联表
CREATE TABLE sys_user_role (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_role_user FOREIGN KEY (user_id) REFERENCES sys_user (id) ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT fk_user_role_role FOREIGN KEY (role_id) REFERENCES sys_role (id) ON DELETE CASCADE ON UPDATE RESTRICT
);

-- 创建唯一索引：一个用户不能重复分配同一个角色
CREATE UNIQUE INDEX idx_sys_user_role_unique ON sys_user_role (user_id, role_id);

-- 创建索引：根据用户ID查询角色
CREATE INDEX idx_sys_user_role_user_id ON sys_user_role (user_id);

-- 创建索引：根据角色ID查询用户
CREATE INDEX idx_sys_user_role_role_id ON sys_user_role (role_id);

-- 添加表注释
COMMENT ON TABLE sys_user_role IS '用户角色关联表';

-- 添加字段注释
COMMENT ON COLUMN sys_user_role.id IS '主键ID';
COMMENT ON COLUMN sys_user_role.user_id IS '用户ID';
COMMENT ON COLUMN sys_user_role.role_id IS '角色ID';
COMMENT ON COLUMN sys_user_role.created_at IS '创建时间';

-- 创建角色菜单关联表
CREATE TABLE sys_role_menu (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL,
    menu_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_role_menu_role FOREIGN KEY (role_id) REFERENCES sys_role (id) ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT fk_role_menu_menu FOREIGN KEY (menu_id) REFERENCES sys_menu (id) ON DELETE CASCADE ON UPDATE RESTRICT
);

-- 创建唯一索引：一个角色不能重复分配同一个菜单
CREATE UNIQUE INDEX idx_sys_role_menu_unique ON sys_role_menu (role_id, menu_id);

-- 创建索引：根据角色ID查询菜单
CREATE INDEX idx_sys_role_menu_role_id ON sys_role_menu (role_id);

-- 创建索引：根据菜单ID查询角色
CREATE INDEX idx_sys_role_menu_menu_id ON sys_role_menu (menu_id);

-- 添加表注释
COMMENT ON TABLE sys_role_menu IS '角色菜单关联表';

-- 添加字段注释
COMMENT ON COLUMN sys_role_menu.id IS '主键ID';
COMMENT ON COLUMN sys_role_menu.role_id IS '角色ID';
COMMENT ON COLUMN sys_role_menu.menu_id IS '菜单ID';
COMMENT ON COLUMN sys_role_menu.created_at IS '创建时间';

-- 创建菜单API关联表
CREATE TABLE sys_menu_api (
    id BIGSERIAL PRIMARY KEY,
    menu_id BIGINT NOT NULL,
    api_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_menu_api_menu FOREIGN KEY (menu_id) REFERENCES sys_menu (id) ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT fk_menu_api_api FOREIGN KEY (api_id) REFERENCES sys_api (id) ON DELETE CASCADE ON UPDATE RESTRICT
);

-- 创建唯一索引：一个菜单不能重复关联同一个API
CREATE UNIQUE INDEX idx_sys_menu_api_unique ON sys_menu_api (menu_id, api_id);

-- 创建索引：根据菜单ID查询API
CREATE INDEX idx_sys_menu_api_menu_id ON sys_menu_api (menu_id);

-- 创建索引：根据API ID查询菜单
CREATE INDEX idx_sys_menu_api_api_id ON sys_menu_api (api_id);

-- 添加表注释
COMMENT ON TABLE sys_menu_api IS '菜单API关联表';

-- 添加字段注释
COMMENT ON COLUMN sys_menu_api.id IS '主键ID';
COMMENT ON COLUMN sys_menu_api.menu_id IS '菜单ID';
COMMENT ON COLUMN sys_menu_api.api_id IS 'API ID';
COMMENT ON COLUMN sys_menu_api.created_at IS '创建时间';
