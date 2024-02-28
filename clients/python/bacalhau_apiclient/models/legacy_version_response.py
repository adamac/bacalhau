# coding: utf-8

"""
    Bacalhau API

    This page is the reference of the Bacalhau REST API. Project docs are available at https://docs.bacalhau.org/. Find more information about Bacalhau at https://github.com/bacalhau-project/bacalhau.  # noqa: E501

    OpenAPI spec version: ${VERSION}
    Contact: team@bacalhau.org
    Generated by: https://github.com/swagger-api/swagger-codegen.git
"""

import pprint
import re  # noqa: F401

import six

class LegacyVersionResponse(object):
    """NOTE: This class is auto generated by the swagger code generator program.

    Do not edit the class manually.
    """
    """
    Attributes:
      swagger_types (dict): The key is attribute name
                            and the value is attribute type.
      attribute_map (dict): The key is attribute name
                            and the value is json key in definition.
    """
    swagger_types = {
        'build_version_info': 'BuildVersionInfo'
    }

    attribute_map = {
        'build_version_info': 'build_version_info'
    }

    def __init__(self, build_version_info=None):  # noqa: E501
        """LegacyVersionResponse - a model defined in Swagger"""  # noqa: E501
        self._build_version_info = None
        self.discriminator = None
        if build_version_info is not None:
            self.build_version_info = build_version_info

    @property
    def build_version_info(self):
        """Gets the build_version_info of this LegacyVersionResponse.  # noqa: E501


        :return: The build_version_info of this LegacyVersionResponse.  # noqa: E501
        :rtype: BuildVersionInfo
        """
        return self._build_version_info

    @build_version_info.setter
    def build_version_info(self, build_version_info):
        """Sets the build_version_info of this LegacyVersionResponse.


        :param build_version_info: The build_version_info of this LegacyVersionResponse.  # noqa: E501
        :type: BuildVersionInfo
        """

        self._build_version_info = build_version_info

    def to_dict(self):
        """Returns the model properties as a dict"""
        result = {}

        for attr, _ in six.iteritems(self.swagger_types):
            value = getattr(self, attr)
            if isinstance(value, list):
                result[attr] = list(map(
                    lambda x: x.to_dict() if hasattr(x, "to_dict") else x,
                    value
                ))
            elif hasattr(value, "to_dict"):
                result[attr] = value.to_dict()
            elif isinstance(value, dict):
                result[attr] = dict(map(
                    lambda item: (item[0], item[1].to_dict())
                    if hasattr(item[1], "to_dict") else item,
                    value.items()
                ))
            else:
                result[attr] = value
        if issubclass(LegacyVersionResponse, dict):
            for key, value in self.items():
                result[key] = value

        return result

    def to_str(self):
        """Returns the string representation of the model"""
        return pprint.pformat(self.to_dict())

    def __repr__(self):
        """For `print` and `pprint`"""
        return self.to_str()

    def __eq__(self, other):
        """Returns true if both objects are equal"""
        if not isinstance(other, LegacyVersionResponse):
            return False

        return self.__dict__ == other.__dict__

    def __ne__(self, other):
        """Returns true if both objects are not equal"""
        return not self == other