FANBOY_VERSION = 1.0.0
FANBOY_LICENSE = Apache License 2.0

FANBOY_DEPENDENCIES = host-go

FANBOY_SITE = $(BR2_EXTERNAL_FANBOY_PATH)/package/fanboy/files
FANBOY_SITE_METHOD = local
BR_NO_CHECK_HASH_FOR += $(FANBOY_SOURCE)

HOST123_GO_TARGET_CC = \
 	CC=$(TARGET_CC) \
 	CXX=$(TARGET_CXX)

HOST123_GO_HOST_CC = \
  CC_FOR_HOST=$(HOSTCC_NOCCACHE) \
  CXX_FOR_HOST=$(HOSTCC_NOCCACHE)

FANBOY_MAKE_PATH = \
  PATH="$(HOST_DIR)/usr/bin:$(PATH)" \
	HOME="/tmp" \
	$(HOST123_GO_HOST_CC) \
	$(HOST123_GO_TARGET_CC)

FANBOY_MAKE_ENV = -v --clean -f --arch=arm --os=linux


define FANBOY_CONFIGURE_CMDS
endef


define FANBOY_BUILD_CMDS
  $(Q)$(call MESSAGE,"Building Fanboy Go application")
  export $(FANBOY_MAKE_PATH) && cd $(@D) && ./go-build $(FANBOY_MAKE_ENV)
endef


define FANBOY_INSTALL_TARGET_CMDS
  $(Q)$(call MESSAGE,"Installing Fanboy Go application")
  $(INSTALL) -m 0755 $(@D)/target/fanboy $(TARGET_DIR)/usr/bin
  mkdir -p $(TARGET_DIR)/var/fanboy
  if [ -d "$(@D)/static" ]; then \
    cp -R $(@D)/static/* $(TARGET_DIR)/var/fanboy; \
    chmod -R 0755 $(TARGET_DIR)/var/fanboy; fi
endef


$(eval $(generic-package))
