let webhooks = [];

const WEBHOOK_EVENTS = ["sent", "opened", "clicked", "submitted", "reported"];

// toggleWebhookType shows the correct fields section for the selected type and
// shows the Test Request button only for the http_api type.
const toggleWebhookType = () => {
    const t = $("#type").val();
    $("#standard_fields").toggle(t === "standard");
    $("#telegram_fields").toggle(t === "telegram");
    $("#api_fields").toggle(t === "http_api");
    $("#testRequestBtn").toggle(t === "http_api");
};

// getSelectedEvents returns the checked event keys as a comma-separated string.
const getSelectedEvents = () => {
    return WEBHOOK_EVENTS.filter((ev) => $("#event_" + ev).is(":checked")).join(",");
};

// setSelectedEvents checks the boxes for the given comma-separated event string.
// An empty string represents the legacy "all events" behavior, so all boxes are
// checked to reflect that.
const setSelectedEvents = (events) => {
    const all = !events || events.trim() === "";
    const list = all ? WEBHOOK_EVENTS : events.split(",").map((e) => e.trim());
    WEBHOOK_EVENTS.forEach((ev) => {
        $("#event_" + ev).prop("checked", all || list.indexOf(ev) !== -1);
    });
};

const dismiss = () => {
    $("#name").val("");
    $("#type").val("standard");
    $("#url").val("");
    $("#secret").val("");
    $("#telegram_bot_token").val("");
    $("#telegram_chat_id").val("");
    $("#telegram_include_username").prop("checked", false);
    $("#telegram_include_password").prop("checked", false);
    $("#telegram_include_tokens").prop("checked", false);
    $("#telegram_username_pattern").val("");
    $("#telegram_min_password_length").val("0");
    $("#telegram_min_token_length").val("0");
    $("#api_url").val("");
    $("#api_method").val("POST");
    $("#api_headers_list").empty();
    $("#api_include_username").prop("checked", false);
    $("#api_include_password").prop("checked", false);
    $("#api_include_tokens").prop("checked", false);
    $("#api_username_pattern").val("");
    $("#api_min_password_length").val("0");
    $("#api_min_token_length").val("0");
    // Default new webhooks to notifying only on Submitted Data
    setSelectedEvents("submitted");
    $("#is_active").prop("checked", false);
    toggleWebhookType();
    $("#flashes").empty();
};

// addAPIHeaderRow appends a new key/value header row to the headers list.
// If key/value are provided the inputs are pre-filled (used when loading an
// existing webhook).
const addAPIHeaderRow = (key, value) => {
    const row = $(`
        <div class="form-group api-header-row" style="display:flex;gap:6px;margin-bottom:4px;">
            <input type="text" class="form-control api-header-key" placeholder="Header name" style="flex:1" value="${escapeHtml(key || '')}" />
            <input type="text" class="form-control api-header-value" placeholder="Header value" style="flex:2" value="${escapeHtml(value || '')}" />
            <button type="button" class="btn btn-danger btn-sm remove-api-header" style="white-space:nowrap;">
                <i class="fa fa-times"></i>
            </button>
        </div>
    `);
    row.find(".remove-api-header").on("click", function() {
        row.remove();
    });
    $("#api_headers_list").append(row);
};

// getAPIHeaders collects all header rows and returns a plain object (or null
// if there are no rows, so the field is omitted from the request body).
const getAPIHeaders = () => {
    const headers = {};
    let hasAny = false;
    $("#api_headers_list .api-header-row").each(function() {
        const k = $(this).find(".api-header-key").val().trim();
        const v = $(this).find(".api-header-value").val().trim();
        if (k) {
            headers[k] = v;
            hasAny = true;
        }
    });
    return hasAny ? JSON.stringify(headers) : "";
};

// setAPIHeaders populates the headers list from a JSON string (as stored in
// the model). Silently ignores invalid JSON.
const setAPIHeaders = (headersJSON) => {
    $("#api_headers_list").empty();
    if (!headersJSON) return;
    let h = {};
    try { h = JSON.parse(headersJSON); } catch (e) { return; }
    Object.keys(h).forEach((k) => addAPIHeaderRow(k, h[k]));
};

const saveWebhook = (id) => {
    const whType = $("#type").val();
    // For HTTP API type, the URL lives in #api_url; for others in #url.
    const urlValue = whType === "http_api" ? $("#api_url").val() : $("#url").val();
    let wh = {
        name: $("#name").val(),
        type: whType,
        url: urlValue,
        secret: $("#secret").val(),
        telegram_bot_token: $("#telegram_bot_token").val(),
        telegram_chat_id: $("#telegram_chat_id").val(),
        telegram_include_username: $("#telegram_include_username").is(":checked"),
        telegram_include_password: $("#telegram_include_password").is(":checked"),
        telegram_include_tokens: $("#telegram_include_tokens").is(":checked"),
        telegram_username_pattern: $("#telegram_username_pattern").val(),
        telegram_min_password_length: parseInt($("#telegram_min_password_length").val(), 10) || 0,
        telegram_min_token_length: parseInt($("#telegram_min_token_length").val(), 10) || 0,
        api_method: $("#api_method").val() || "POST",
        api_headers: getAPIHeaders(),
        api_include_username: $("#api_include_username").is(":checked"),
        api_include_password: $("#api_include_password").is(":checked"),
        api_include_tokens: $("#api_include_tokens").is(":checked"),
        api_username_pattern: $("#api_username_pattern").val(),
        api_min_password_length: parseInt($("#api_min_password_length").val(), 10) || 0,
        api_min_token_length: parseInt($("#api_min_token_length").val(), 10) || 0,
        events: getSelectedEvents(),
        is_active: $("#is_active").is(":checked"),
    };
    if (id != -1) {
        wh.id = parseInt(id);
        api.webhookId.put(wh)
            .success(function(data) {
                dismiss();
                load();
                $("#modal").modal("hide");
                successFlash(`Webhook "${escapeHtml(wh.name)}" has been updated successfully!`);
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.webhooks.post(wh)
            .success(function(data) {
                load();
                dismiss();
                $("#modal").modal("hide");
                successFlash(`Webhook "${escapeHtml(wh.name)}" has been created successfully!`);
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    }
};

const load = () => {
    $("#webhookTable").hide();
    $("#loading").show();
    api.webhooks.get()
        .success((whs) => {
            webhooks = whs;
            $("#loading").hide()
            $("#webhookTable").show()
            let webhookTable = $("#webhookTable").DataTable({
                destroy: true,
                columnDefs: [{
                    orderable: false,
                    targets: "no-sort"
                }]
            });
            webhookTable.clear();
            $.each(webhooks, (i, webhook) => {
                webhookTable.row.add([
                    escapeHtml(webhook.name),
                    escapeHtml(webhook.url),
                    escapeHtml(webhook.is_active),
                    `
                      <div class="pull-right">
                        <button class="btn btn-primary ping_button" data-webhook-id="${webhook.id}">
                          Ping
                        </button>
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-webhook-id="${webhook.id}">
                          <i class="fa fa-pencil"></i>
                        </button>
                        <button class="btn btn-danger delete_button" data-webhook-id="${webhook.id}">
                          <i class="fa fa-trash-o"></i>
                        </button>
                      </div>
                    `
                ]).draw()
            })
        })
        .error(() => {
            errorFlash("Error fetching webhooks")
        })
};

const editWebhook = (id) => {
    $("#modalSubmit").unbind("click").click(() => {
        saveWebhook(id);
    });
    if (id !== -1) {
        $("#webhookModalLabel").text("Edit Webhook")
        api.webhookId.get(id)
          .success(function(wh) {
              const t = wh.type || "standard";
              $("#name").val(wh.name);
              $("#type").val(t);
              // Populate URL into the correct field based on type
              if (t === "http_api") {
                  $("#api_url").val(wh.url);
                  $("#url").val("");
              } else {
                  $("#url").val(wh.url);
                  $("#api_url").val("");
              }
              $("#secret").val(wh.secret);
              $("#telegram_bot_token").val(wh.telegram_bot_token);
              $("#telegram_chat_id").val(wh.telegram_chat_id);
              $("#telegram_include_username").prop("checked", wh.telegram_include_username);
              $("#telegram_include_password").prop("checked", wh.telegram_include_password);
              $("#telegram_include_tokens").prop("checked", wh.telegram_include_tokens);
              $("#telegram_username_pattern").val(wh.telegram_username_pattern || "");
              $("#telegram_min_password_length").val(wh.telegram_min_password_length || 0);
              $("#telegram_min_token_length").val(wh.telegram_min_token_length || 0);
              $("#api_method").val(wh.api_method || "POST");
              setAPIHeaders(wh.api_headers || "");
              $("#api_include_username").prop("checked", wh.api_include_username);
              $("#api_include_password").prop("checked", wh.api_include_password);
              $("#api_include_tokens").prop("checked", wh.api_include_tokens);
              $("#api_username_pattern").val(wh.api_username_pattern || "");
              $("#api_min_password_length").val(wh.api_min_password_length || 0);
              $("#api_min_token_length").val(wh.api_min_token_length || 0);
              setSelectedEvents(wh.events);
              $("#is_active").prop("checked", wh.is_active);
              toggleWebhookType();
          })
          .error(function () {
              errorFlash("Error fetching webhook")
          });
    } else {
        // Reset the form to defaults (standard type, Submitted Data only)
        dismiss();
        $("#webhookModalLabel").text("New Webhook")
    }
};

const deleteWebhook = (id) => {
    var wh = webhooks.find(x => x.id == id);
    if (!wh) {
        return;
    }
    Swal.fire({
        title: "Are you sure?",
        text: `This will delete the webhook '${escapeHtml(wh.name)}'`,
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise((resolve, reject) => {
                api.webhookId.delete(id)
                    .success((msg) => {
                        resolve()
                    })
                    .error((data) => {
                        reject(data.responseJSON.message)
                    })
            })
            .catch(error => {
                Swal.showValidationMessage(error)
              })
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire(
                "Webhook Deleted!",
                `The webhook has been deleted!`,
                "success"
            );
        }
        $("button:contains('OK')").on("click", function() {
            location.reload();
        })
    })
};

// testRequest collects the current form values and sends a test event to the
// configured API endpoint without saving the webhook.
const testRequest = (btn) => {
    const config = {
        type: "http_api",
        url: $("#api_url").val(),
        api_method: $("#api_method").val() || "POST",
        api_headers: getAPIHeaders(),
        api_include_username: $("#api_include_username").is(":checked"),
        api_include_password: $("#api_include_password").is(":checked"),
        api_include_tokens: $("#api_include_tokens").is(":checked"),
        api_username_pattern: $("#api_username_pattern").val(),
        api_min_password_length: parseInt($("#api_min_password_length").val(), 10) || 0,
        api_min_token_length: parseInt($("#api_min_token_length").val(), 10) || 0,
    };
    if (!config.url) {
        modalError("URL is required to send a test request.");
        return;
    }
    btn.disabled = true;
    api.webhookId.testRequest(config)
        .success(function() {
            btn.disabled = false;
            successFlash("Test request sent successfully.");
        })
        .error(function(data) {
            btn.disabled = false;
            modalError("Test request failed: " + escapeHtml(data.responseJSON.message));
        });
};

const pingUrl = (btn, whId) => {
    dismiss();
    btn.disabled = true;
    api.webhookId.ping(whId)
        .success(function(wh) {
            btn.disabled = false;
            successFlash(`Ping of "${escapeHtml(wh.name)}" webhook succeeded.`);
        })
        .error(function(data) {
            btn.disabled = false;
            var wh = webhooks.find(x => x.id == whId);
            if (!wh) {
                return
            }
            errorFlash(`Ping of "${escapeHtml(wh.name)}" webhook failed: "${escapeHtml(data.responseJSON.message)}"`)
        });
};

$(document).ready(function() {
    load();
    $("#modal").on("hide.bs.modal", function() {
        dismiss();
    });
    $("#new_button").on("click", function() {
        editWebhook(-1);
    });
    $("#type").on("change", function() {
        toggleWebhookType();
    });
    $("#add_api_header").on("click", function() {
        addAPIHeaderRow("", "");
    });
    $("#testRequestBtn").on("click", function() {
        testRequest(this);
    });
    $("#webhookTable").on("click", ".edit_button", function(e) {
        editWebhook($(this).attr("data-webhook-id"));
    });
    $("#webhookTable").on("click", ".delete_button", function(e) {
        deleteWebhook($(this).attr("data-webhook-id"));
    });
    $("#webhookTable").on("click", ".ping_button", function(e) {
        pingUrl(e.currentTarget, e.currentTarget.dataset.webhookId);
    });
});
