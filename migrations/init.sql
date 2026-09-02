-- Reference schema for Bubble White.
--
-- The Go app auto-migrates these tables itself on boot (see
-- config/database.go -> AutoMigrate), so running this file manually is
-- OPTIONAL — it's here for reference, and so `docker compose up` gives you
-- an inspectable schema immediately via phpMyAdmin even before the app
-- has started once.

CREATE TABLE IF NOT EXISTS permissions (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  slug        VARCHAR(100) NOT NULL UNIQUE,
  description VARCHAR(255),
  `group`     VARCHAR(50)
);

CREATE TABLE IF NOT EXISTS roles (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name        VARCHAR(100) NOT NULL,
  slug        VARCHAR(100) NOT NULL UNIQUE,
  description VARCHAR(255),
  permissions JSON,
  is_system   BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS users (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name          VARCHAR(150) NOT NULL,
  email         VARCHAR(150) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role_id       BIGINT UNSIGNED NOT NULL,
  is_active     BOOLEAN DEFAULT TRUE,
  created_at    DATETIME,
  updated_at    DATETIME,
  INDEX idx_users_role_id (role_id),
  FOREIGN KEY (role_id) REFERENCES roles(id)
);

CREATE TABLE IF NOT EXISTS categories (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name       VARCHAR(100) NOT NULL,
  slug       VARCHAR(100) NOT NULL UNIQUE,
  image      VARCHAR(500),
  created_at DATETIME,
  updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS products (
  id          VARCHAR(50) PRIMARY KEY,
  name        VARCHAR(255) NOT NULL,
  price       DECIMAL(10,2) NOT NULL,
  compare_at  DECIMAL(10,2),
  category_id VARCHAR(50) NOT NULL, -- stores the category's SLUG, not categories.id
  badge       VARCHAR(50),
  featured    BOOLEAN DEFAULT FALSE,
  sizes       JSON,
  images      JSON,
  description TEXT,
  created_at  DATETIME,
  updated_at  DATETIME,
  INDEX idx_products_category_id (category_id),
  INDEX idx_products_featured (featured),
  FOREIGN KEY (category_id) REFERENCES categories(slug)
);

CREATE TABLE IF NOT EXISTS contact_messages (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name       VARCHAR(150) NOT NULL,
  email      VARCHAR(150) NOT NULL,
  subject    VARCHAR(255),
  message    TEXT NOT NULL,
  is_read    BOOLEAN DEFAULT FALSE,
  created_at DATETIME
);

CREATE TABLE IF NOT EXISTS settings (
  id              BIGINT UNSIGNED PRIMARY KEY,
  company_name    VARCHAR(150),
  company_detail  TEXT,
  contact_email   VARCHAR(150),
  contact_phone   VARCHAR(50),
  contact_address VARCHAR(255),
  working_hours   VARCHAR(150),
  facebook_url    VARCHAR(255),
  instagram_url   VARCHAR(255),
  tiktok_url      VARCHAR(255),
  telegram_url    VARCHAR(255),
  logo_url        VARCHAR(500),
  updated_at      DATETIME
);
