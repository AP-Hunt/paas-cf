require 'securerandom'
require 'time'
require 'set'
require 'uaa'

require 'pp'

# Utility class to create or delete admin users.
#
# Manages  users members from the groups listed in
# UaaSyncAdminUsers::DEFAULT_ADMIN_GROUPS
#
class UaaSyncAdminUsers
  attr_accessor :ua, :token_issuer, :admin_groups

  DEFAULT_ADMIN_GROUPS = [
    "cloud_controller.admin",
    "uaa.admin",
    "scim.read",
    "scim.write",
    "doppler.firehose"
  ]

  HOURS_TO_KEEP_TEST_USERS = 2

  # Initialise the class with to the given UAA target with the given
  # credentials.
  #
  # Params:
  # - target: UAA target
  # - admin_client: Admin client name for UAA with permissions to manage users
  # - admin_password: Admin client password
  # - options: Hash of options. Examples and defaults
  #   - skip_ssl_validation: default=false
  #   - extra_admin_groups: default=[] Additional admin groups apart of DEFAULT_ADMIN_GROUPS
  #   - log_level: default: :warn. Options: :debug, :trace, :warn
  def initialize(target, admin_client, admin_password, options=nil)
    @target = target
    @admin_client = admin_client
    @admin_password = admin_password
    @options = options

    self.admin_groups = DEFAULT_ADMIN_GROUPS + (options[:extra_admin_groups] || [])

    self.token_issuer = CF::UAA::TokenIssuer.new(@target, @admin_client, @admin_password, @options)
    self.token_issuer.logger = self.get_logger()
  end

  # Returns the logger for this object.
  # Set by options[:log_level] = {:debug, :trace, :warn} or $UAA_LOG_LEVEL
  def get_logger()
    if @logger.nil?
      if ENV['UAA_LOG_LEVEL']
        log_level = ENV['UAA_LOG_LEVEL'].strip.downcase.to_sym
      else
        log_level = @options[:log_level] || :info
      end
      @logger = Logger.new($stdout)
      @logger.level = Logger::Severity.const_get(log_level.to_s.upcase)
    end
    @logger
  end

  def token=(token)
    @token = token
  end

  def token()
    raise "Token not initialised. Did you call .request_token()?" if @token.nil?
    return @token
  end

  def auth_header()
    return "#{self.token['token_type']} #{self.token['access_token']}"
  end

  def get_user_by_username(username)
    type = :user
    query = { filter: "username eq \"#{username}\"" }
    self.ua.all_pages(type, query)[0]
  end

  def get_user_by_id(id)
    type = :user
    query = { filter: "id eq \"#{id}\"" }
    self.ua.all_pages(type, query)[0]
  end

  # Creates a user with the given username, email and password
  # params:
  # - user: Hash as {username: ..., password: ..., email:...}
  def create_user(user)
    self.get_logger.info("Creating user #{user[:username]}")
    info = {
      userName: user[:username],
      password: user[:password],
      emails: [{value: user[:email]}],
    }
    self.ua.add(:user, info)
  end

  def get_group_by_name(groupname)
    type = :group
    query = { filter: "#{self.ua.name_attr(type)} eq \"#{groupname}\"" }
    self.ua.all_pages(type, query)[0]
  end

  # Authenticates the client with the UAA server and requests a new token.
  def request_token()
    self.token = self.token_issuer.client_credentials_grant().info
    self.ua = CF::UAA::Scim.new(@target, self.auth_header, @options)
    self.ua.logger = self.get_logger()
    return self
  end

  # Creates the given users and adds them to the Admin users
  # Params:
  # - users: List of users as hashes {username: ..., email: ... , password: ... }
  #   Is password is undefined, it will generate one.
  def update_admin_users(users)
    created_users = []
    deleted_users = []

    # Ensure we always keep admin
    users = users.clone()
    users << {
      username: "admin",
      email: "admin"
    }

    # Get all the admin groups info
    groups_info = {}
    self.admin_groups.each { |group_name|
      groups_info[group_name] = get_group_by_name(group_name)
      groups_info[group_name]["members"] ||= []
    }

    # Get/Create users if required
    users_info = {}
    users.each{ |user|
      user_info = self.get_user_by_username(user[:username])
      if user_info.nil?
        user[:password] ||= SecureRandom.hex
        user_info = self.create_user(user)
        created_users << user
      end
      users_info[user[:username]] = user_info
    }

    # Add users to groups if required
    self.admin_groups.each { |group_name|
      group_info_to_update = nil
      member_set = Set.new
      groups_info[group_name]["members"].each { |m|
        member_set << m["value"]
      }
      users.each{ |user|
        user_info = users_info[user[:username]]
        if not member_set.include? user_info["id"]
          get_logger.info("Adding user #{user_info["username"]} to group #{group_name}")
          member_set << user_info["id"]
          group_info_to_update ||= groups_info[group_name].clone()
          group_info_to_update["members"] = member_set.to_a
        end
      }
      if group_info_to_update
        get_logger.info("Updating group #{group_name}")
        self.ua.put(:group, group_info_to_update)
      end
    }

    # Remove users in groups that should not be there
    allowed_user_ids = users_info.map { |_, v| v["id"] }
    existing_user_ids = Set.new
    groups_info.each { |_, group_info|
      group_info["members"].each{ |m| existing_user_ids << m["value"] }
    }

    to_delete_user_ids = existing_user_ids - allowed_user_ids
    to_delete_user_ids.each { |user_id|
      user_info = self.get_user_by_id(user_id)
      if user_info.nil?
        raise "User #{user_id} not found"
      end

      if Time.parse(user_info["meta"]["created"]) < (Time.now - HOURS_TO_KEEP_TEST_USERS * 60 * 60)
        get_logger.info("Deleting admin user #{user_info["userName"]} which is not in the list.")
        ua.delete(:user, user_id)
        deleted_users << { username: user_info["username"], email: user_info["emails"][0]["value"] }
      else
        get_logger.info("Not deleting user #{user_info["username"]} created in the last #{HOURS_TO_KEEP_TEST_USERS} hours.")
      end
    }

    return created_users, deleted_users

  end
end
